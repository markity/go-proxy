package main

import (
	"errors"
	"go-proxy/comm"
	"log"
	"net"

	"github.com/pion/dtls"
	"github.com/songgao/water"
)

func main() {
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			hintStr := string(hint)
			pwd, ok := UserMap[hintStr]
			if !ok {
				return nil, errors.New("unregistered username")
			}
			return []byte(pwd), nil
		},
		CipherSuites:   []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		MTU:            1400,
		ConnectTimeout: &ConnetTimeout,
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

	ipv4Poll := NewAddrPool("10.8.0.0/16")

	for {
		// accept
		c, err := ln.Accept()
		if err != nil {
			log.Fatalf("Accept error: %v\n", err)
		}
		go func() {
			forClient := ipv4Poll.Next()
			forServer := ipv4Poll.Next()
			defer func() {
				ipv4Poll.Release(forClient)
				ipv4Poll.Release(forServer)
				c.Close()
			}()

			tun, err := water.New(water.Config{DeviceType: water.TUN})
			if err != nil {
				log.Fatalf("failed to create tun: %v\n", err)
			}
			defer tun.Close()

			MustIPCmd("link", "set", tun.Name(), "up", "mtu", "1300")
			MustIPCmd("addr", "add", forServer.String(), "dev", tun.Name())
			MustIPCmd("route", "add", forClient.String()+"/32", "via", forServer.String())

			// 发送ip dispatch包, 然后开始转发数据包
			c.Write(comm.NewIPDispatchPacket(forClient, forServer).Pack())

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

			// tun reader
			go func() {
				for {
					buf := make([]byte, 1400, 1400)
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
		}()

	}
}
