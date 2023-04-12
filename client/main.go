package main

import (
	"encoding/json"
	"fmt"
	"go-proxy/comm"
	"log"
	"net"
	"time"

	"github.com/pion/dtls"
	"github.com/songgao/water"
	"inet.af/netaddr"
)

var ServerIP = "127.0.0.1"
var ServerPort = 8000
var ConnetTimeout = time.Second * 3

func main() {
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			log.Printf("Server's hint: %s \n", string(hint))
			return []byte("mark2004119"), nil
		},
		PSKIdentityHint: []byte("markity"),
		CipherSuites:    []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		MTU:             1400,
		ConnectTimeout:  &ConnetTimeout,
	}

	addr, err := net.ResolveUDPAddr("udp", ServerIP+":"+fmt.Sprint(ServerPort))
	if err != nil {
		log.Fatalf("failed to ResolveUDPAddr: %v\n", err)
	}

	log.Println("dialing to:", addr)
	c, err := dtls.Dial("udp", addr, config)
	if err != nil {
		log.Fatalf("failed to dial to server: %v\n", err)
	}
	defer c.Close()

	var dispatchChan = make(chan *comm.IPDispatchPacket)
	var errStringChan = make(chan string)

	// conn reader
	go func() {
		buf := make([]byte, 1400, 1400)
		for {
			c.SetReadDeadline(time.Now().Add(time.Second * 3))
			n, err := c.Read(buf)
			if err != nil {
				errStringChan <- err.Error()
				return
			}

			switch comm.ParsePacket(buf[:n]) {
			case comm.IPDispatchPacketType:
				var p comm.IPDispatchPacket
				json.Unmarshal(buf[:n], &p)
				dispatchChan <- &p
			default:
				errStringChan <- "protocol error: wrong packet type"
			}
		}
	}()

	var forClientIP, forServerIP netaddr.IP

	select {
	case m := <-dispatchChan:
		// 检查两个ip是否合法
		forClientIP, err = netaddr.ParseIP(m.ForClient)
		if err != nil {
			log.Fatalf("protocol error: wrong response\n")
		}
		forServerIP, err = netaddr.ParseIP(m.ForServer)
		if err != nil {
			log.Fatalf("protocol error: wrong response\n")
		}
	case m := <-errStringChan:
		log.Fatalf("failed to dial to server: %v\n", m)
		return
	}
	fmt.Println(123)

	// 开tun
	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Fatalf("failed to create tun device: %v\n", err)
	}
	MustIPCmd("link", "set", tun.Name(), "up", "mtu", "1300")
	MustIPCmd("addr", "add", forClientIP.String(), "peer", forClientIP.String(), "dev", tun.Name())
	time.Sleep(time.Second * 3)
	// 通过脚本执行
	shFmt := `DEFAULT_GW=$(ip route|grep default|cut -d' ' -f3)
ip route add %v/32 via $DEFAULT_GW
ip route add 0.0.0.0/1 via %v
ip route add 128.0.0.0/1 via %v`
	MustShCmd("-c", fmt.Sprintf(shFmt, ServerIP, forServerIP.String(), forServerIP.String()))

	var tunChan = make(chan []byte)
	var connectionMessageChan = make(chan interface{})
	errStringChan = make(chan string)

	// tun reader
	go func() {
		for {
			buf := make([]byte, 1300, 1300)
			n, err := tun.Read(buf)
			if err != nil {
				// 出现err的原因是tun被关了
				return
			}
			tunChan <- buf[:n]
		}
	}()

	// connection reader
	go func() {
		buf := make([]byte, 1400, 1400)
		// 3s收不到包就关了
		for {
			c.SetReadDeadline(time.Now().Add(time.Second * 3))
			_, err := c.Read(buf)
			if err != nil {
				errStringChan <- err.Error()
				return
			}

			switch comm.ParsePacket(buf) {
			case comm.HeartPacketType:
				connectionMessageChan <- comm.NewHeartPacket()
			case comm.IPDispatchPacketType:
				var p comm.IPDispatchPacket
				json.Unmarshal(buf, &p)
				connectionMessageChan <- &p
			case comm.IPPacketPacketType:
				var p comm.IPPacketPacket
				json.Unmarshal(buf, &p)
				connectionMessageChan <- &p
			}
		}
	}()

	select {
	case m := <-errStringChan:
		log.Printf("connection read error, now close the tunnel: %v\n", m)
		return
	case m := <-connectionMessageChan:
		switch v := m.(type) {
		case *comm.HeartPacket:
			// empty
		case *comm.IPPacketPacket:
			tun.Write([]byte(v.Data))
		}
	case m := <-tunChan:
		_, err := c.Write(m)
		if err != nil {
			// unreachable
			panic(err)
		}
	case <-time.NewTicker(time.Second).C:
		_, err := c.Write(comm.NewHeartPacket().Pack())
		if err != nil {
			// 出错的原因是c被关了
			return
		}
		time.Sleep(time.Second)
	}
}
