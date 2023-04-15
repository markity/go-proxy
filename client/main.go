package main

import (
	"fmt"
	"go-proxy/comm"
	"log"
	"net"
	"time"

	"github.com/pion/dtls"
	"github.com/songgao/water"
)

func main() {
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			return []byte(Password), nil
		},
		PSKIdentityHint: []byte(Username),
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

	// 读取ip dispatch包
	packetBytes := make([]byte, 1400, 1400)
	c.SetReadDeadline(time.Now().Add(ReadTimeout))
	n, err := c.Read(packetBytes)
	if err != nil {
		log.Fatalf("failed to read: %v\n", err)
	}

	// 读到的第一个数据包应该是ip dispatch包
	packet := comm.ParsePacket(packetBytes[:n])
	ipDispatchPacket, ok := packet.(*comm.IPDispatchPacket)
	if packet == nil || !ok {
		log.Fatalf("protocol error\n")
	}

	// 开tun
	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Fatalf("failed to create tun device: %v\n", err)
	}
	defer tun.Close()

	comm.MustIPCmd("link", "set", tun.Name(), "up", "mtu", "1300")
	comm.MustIPCmd("addr", "add", ipDispatchPacket.ForClient, "dev", tun.Name())

	fmt.Println("creating route table...")
	// 通过脚本执行
	shFmt := `DEFAULT_GW=$(ip route|grep default|cut -d' ' -f3)
ip route add %v via $DEFAULT_GW
ip route add 0.0.0.0/1 dev %v
ip route add 128.0.0.0/1 dev %v`
	comm.MustShCmd("-c", fmt.Sprintf(shFmt, ServerIP, tun.Name(), tun.Name()))

	fmt.Printf("transferring data...\n")

	// tun reader
	go func() {
		for {
			buf := make([]byte, 1300, 1300)
			n, err := tun.Read(buf)
			if err != nil {
				panic(err)
			}
			println("write conn")
			_, err = c.Write(buf[:n])
			if err != nil {
				panic(err)
			}
		}
	}()

	// connection reader
	go func() {
		for {
			buf := make([]byte, 1400, 1400)
			n, err := c.Read(buf)
			if err != nil {
				panic(err)
			}
			println("write tun")
			_, err = tun.Write(buf[:n])
			if err != nil {
				panic(err)
			}
		}

	}()

	select {}
}
