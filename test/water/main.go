package main

import (
	"fmt"
	"log"

	"github.com/songgao/water"
)

func main() {
	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Fatalf("failed to create tun device: %v\n", err)
	}

	fmt.Println("created tun:", tun.Name())

	// ifconfig tun0 10.8.0.2/16 mtu 1400 up
	// route add 123.123.123.123 tun0

	b := make([]byte, 1500, 1500)
	for {
		n, err := tun.Read(b)
		if err != nil {
			panic(err)
		}
		fmt.Println(b[:n])
	}
}
