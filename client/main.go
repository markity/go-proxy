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

	timeTickChan := time.Tick(time.Second)
	errorChan := make(chan error)
	tunReadChan := make(chan []byte)
	tunReadExitChan := make(chan struct{})
	connectionReadChan := make(chan interface{})
	connectionReadExitChan := make(chan struct{})

	// tun reader
	go func() {
		buf := make([]byte, 1300, 1300)
		for {
			n, err := tun.Read(buf)
			if err != nil {
				errorChan <- err
				<-tunReadExitChan
				return
			}

			copyBuf := make([]byte, n, n)
			copy(copyBuf, buf[:n])
			select {
			case tunReadChan <- copyBuf:
			case <-tunReadExitChan:
				return
			}
		}
	}()

	// connection reader
	go func() {
		buf := make([]byte, 1400, 1400)
		for {
			c.SetReadDeadline(time.Now().Add(ReadTimeout))
			n, err := c.Read(buf)
			if err != nil {
				errorChan <- err
				<-connectionReadExitChan
				return
			}

			select {
			case connectionReadChan <- comm.ParsePacket(buf[:n]):
			case <-connectionReadExitChan:
				return
			}
		}
	}()

	for {
		select {
		case <-timeTickChan:
			_, err := c.Write(comm.NewHeartPacket().Pack())
			if err != nil {
				tunReadExitChan <- struct{}{}
				connectionReadExitChan <- struct{}{}
				log.Printf("failed to write to connection: %v\n", err)
				return
			}
		case err := <-errorChan:
			tunReadExitChan <- struct{}{}
			connectionReadExitChan <- struct{}{}
			log.Printf("error happened: %v", err)
			return
		case ipPacketContent := <-tunReadChan:
			_, err := c.Write(comm.NewIPPacketPacket(string(ipPacketContent)).Pack())
			if err != nil {
				log.Printf("failed to write to connection: %v\n", err)
				tunReadExitChan <- struct{}{}
				connectionReadExitChan <- struct{}{}
				return
			}
		case msg := <-connectionReadChan:
			switch v := msg.(type) {
			case *comm.IPPacketPacket:
				_, err := tun.Write([]byte(v.Data))
				if err != nil {
					log.Printf("failed to write to tun: %v\n", err)
					tunReadExitChan <- struct{}{}
					connectionReadExitChan <- struct{}{}
					return
				}
			case *comm.HeartPacket:
			default:
				log.Printf("protocol error: unexpected packet received\n")
				tunReadExitChan <- struct{}{}
				connectionReadExitChan <- struct{}{}
				return
			}
		}
	}
}
