package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/pion/dtls"
	"github.com/songgao/water"
)

var ServerIP = "162.14.208.15"
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

	// 开tun
	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Fatalf("failed to create tun device: %v\n", err)
	}
	// ifconfig tun0 10.8.0.2/16 mtu %d up
	MustIfconfigCmd(tun.Name(), "10.8.0.2/16", "mtu", "1300", "up")

	fmt.Println("creating route table...")
	// 通过脚本执行
	shFmt := `DEFAULT_GW=$(ip route|grep default|cut -d' ' -f3)
ip route add %v via $DEFAULT_GW
ip route add 0.0.0.0/1 dev %v
ip route add 128.0.0.0/1 dev %v`
	MustShCmd("-c", fmt.Sprintf(shFmt, ServerIP, tun.Name(), tun.Name()))

	fmt.Printf("transferring data...\n")

	// tun reader
	go func() {
		for {
			buf := make([]byte, 1300, 1300)
			n, err := tun.Read(buf)
			if err != nil {
				return
			}
			c.Write(buf[:n])
		}
	}()

	// connection reader
	go func() {
		buf := make([]byte, 1400, 1400)
		n, err := c.Read(buf)
		if err != nil {
			return
		}
		tun.Write(buf[:n])
	}()

	select {}
}
