package main

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/pion/dtls"
	"github.com/songgao/water"
)

var Logined = false
var Mutex sync.Mutex
var ConnetTimeout = time.Second * 3

func main() {
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			log.Printf("Client's hint: %s \n", string(hint))
			return []byte("mark2004119"), nil
		},
		PSKIdentityHint: []byte("markity"),
		CipherSuites:    []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		MTU:             1400,
		ConnectTimeout:  &ConnetTimeout,
	}

	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:8000")
	if err != nil {
		log.Fatalf("failed to ResolveUDPAddr: %v\n", err)
	}

	log.Println("listening on:", addr)

	ln, err := dtls.Listen("udp", addr, config)
	if err != nil {
		log.Fatalf("failed to Listen: %v\n", err)
	}

	// accept
	c, err := ln.Accept()
	if err != nil {
		log.Fatalf("Accept error: %v\n", err)
	}

	// 对udp不考虑write超时的情况

	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Fatalf("failed to create tun: %v\n", err)
	}
	defer tun.Close()

	MustIfconfigCmd(tun.Name(), "10.8.0.1/16", "mtu", "1300", "up")

	// conn reader
	go func() {
		buf := make([]byte, 1400, 1400)
		n, err := c.Read(buf)
		if err != nil {
			return
		}
		tun.Write(buf[:n])
	}()

	// tun reader
	go func() {
		for {
			buf := make([]byte, 1400, 1400)
			n, err := tun.Read(buf)
			if err != nil {
				return
			}
			c.Write(buf[:n])
		}
	}()

	select {}
}
