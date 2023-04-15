package main

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/songgao/water"
)

func main() {
	// 创建tun设备
	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Fatalf("failed to create tun device: %v\n", err)
	}

	fmt.Println("created tun:", tun.Name())

	// 启动网卡
	if err := exec.Command("ip", "link", "set", "up", "dev", tun.Name()).Run(); err != nil {
		log.Fatal(err)
	}

	// 配置ip
	if err := exec.Command("ip", "addr", "add", "10.8.0.1/24", "dev", tun.Name()).Run(); err != nil {
		log.Fatal(err)
	}

	b := make([]byte, 1500, 1500)
	for {
		n, err := tun.Read(b)
		if err != nil {
			// unreachable
			panic(err)
		}
		fmt.Println(b[:n])
	}
}
