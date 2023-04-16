package main

import (
	"fmt"
	"go-proxy/comm"
	"log"
	"net"
	"time"

	"github.com/pion/dtls"
	"github.com/songgao/water"
	"inet.af/netaddr"
)

func main() {
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			return []byte(Password), nil
		},
		PSKIdentityHint: []byte(Username),
		CipherSuites:    []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		MTU:             1500,
		ConnectTimeout:  &ConnetTimeout,
	}

	addr, err := net.ResolveUDPAddr("udp", ServerIP+":"+fmt.Sprint(ServerPort))
	if err != nil {
		log.Printf("failed to ResolveUDPAddr: %v\n", err)
		return
	}

	log.Println("dialing to:", addr)
	c, err := dtls.Dial("udp", addr, config)
	if err != nil {
		log.Printf("failed to dial to server: %v\n", err)
		return
	}
	defer c.Close()

	// 读取ip dispatch包
	packetBytes := make([]byte, 1500, 1500)
	c.SetReadDeadline(time.Now().Add(ReadTimeout))
	n, err := c.Read(packetBytes)
	if err != nil {
		log.Fatalf("failed to read: %v\n", err)
	}

	// 读到的第一个数据包应该是ip dispatch包
	ipString := string(packetBytes[:n])
	ipForClient, err := netaddr.ParseIP(ipString)
	if err != nil || !ipForClient.Is4() {
		log.Fatalf("protocol error: unexpected ip dispatch packet read")
	}

	fmt.Printf("succeed to fetch ip: %v\n", ipForClient)

	// 开tun
	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Printf("failed to create tun device: %v\n", err)
		return
	}
	defer tun.Close()

	comm.MustIPCmd("link", "set", tun.Name(), "up", "mtu", "1500")
	comm.MustIPCmd("addr", "add", ipString, "dev", tun.Name())

	fmt.Println("creating route table...")
	// 通过脚本执行
	shFmt := `DEFAULT_GW=$(ip route|grep default|cut -d' ' -f3)
ip route add %v via $DEFAULT_GW
ip route add 0.0.0.0/1 dev %v
ip route add 128.0.0.0/1 dev %v`
	comm.MustShCmd("-c", fmt.Sprintf(shFmt, ServerIP, tun.Name(), tun.Name()))

	fmt.Printf("transferring data...\n")

	timeTickChan := time.NewTicker(HeartInterval).C
	errorChan := make(chan error)
	tunReadChan := make(chan []byte)
	tunReadExitChan := make(chan struct{})
	connectionReadChan := make(chan []byte)
	connectionReadExitChan := make(chan struct{})

	// tun reader
	go func() {
		buf := make([]byte, 1500, 1500)
		for {
			n, err := tun.Read(buf)
			if err != nil {
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
		buf := make([]byte, 1500, 1500)
		for {
			c.SetReadDeadline(time.Now().Add(ReadTimeout))
			n, err := c.Read(buf)
			if err != nil {
				println("error happened www")
				errorChan <- err
				<-connectionReadExitChan
				return
			}

			copyBuf := make([]byte, n, n)
			copy(copyBuf, buf[:n])
			select {
			case connectionReadChan <- copyBuf:
			case <-connectionReadExitChan:
				return
			}
		}
	}()

	for {
		println("select")
		select {
		case <-timeTickChan:
			fmt.Println("tick")
			_, err := c.Write(comm.HeartMagicPacket)
			if err != nil {
				log.Fatalf("failed to write to connection: %v\n", err)
			}
		case err := <-errorChan:
			fmt.Println("error")
			log.Fatalf("error happened: %v\n", err)
		case ipPacketContent := <-tunReadChan:
			fmt.Println("ip packet")
			_, err := c.Write(ipPacketContent)
			fmt.Println(ipPacketContent)
			if err != nil {
				log.Fatalf("failed to write to connection: %v\n", err)
			}
		case msg := <-connectionReadChan:
			if string(msg) == string(comm.HeartMagicPacket) {
				println("heart packet")
			} else {
				fmt.Println("ip packet")
				_, err := tun.Write(msg)
				if err != nil {
					log.Fatalf("failed to write to tun: %v\n", err)
				}
			}
		}
	}
}
