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

	// ip link set tun1 up
	// route add 0.0.0.0/1 tun1
	// route add 128.0.0.0.1/1 tun1

	b := make([]byte, 1500, 1500)
	for {
		n, err := tun.Read(b)
		if err != nil {
			panic(err)
		}
		fmt.Println("----------")
		fmt.Println(b[:n])
		fmt.Println("----------")
	}
}
