package main

import (
	"fmt"
	"log"
	"net"

	"github.com/songgao/water"
)

func main() {
	tcpListener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 8000})
	if err != nil {
		log.Fatalf("failed to ListenTCP: %v\n", err)
	}

	tcpConn, err := tcpListener.AcceptTCP()
	if err != nil {
		log.Fatalf("failed to AcceptTcp: %v\n", err)
	}

	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Fatalf("failed to create tun: %v\n", err)
	}

	// ip link set tun0 up

	go func() {
		b := make([]byte, 1500, 1500)
		n, err := tcpConn.Read(b)
		if err != nil {
			panic(err)
		}
		fmt.Println("read conn: %v\n", n)
	}()

}
