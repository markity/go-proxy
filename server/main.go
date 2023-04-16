package main

import (
	"errors"
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
			hintStr := string(hint)
			pwd, ok := UserMap[hintStr]
			if !ok {
				return nil, errors.New("unregistered username")
			}
			return []byte(pwd), nil
		},
		CipherSuites:   []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		MTU:            1400,
		ConnectTimeout: &ConnectTimeout,
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
			log.Printf("Accept error, continuing: %v\n", err)
			continue
		}

		fmt.Println("succeed to Accept")

		go func() {
			forServer := ipv4Poll.Next()
			forClient := ipv4Poll.Next()
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

			comm.MustIPCmd("link", "set", tun.Name(), "up", "mtu", "1500")
			comm.MustIPCmd("addr", "add", forServer.String(), "dev", tun.Name())
			comm.MustIPCmd("route", "add", forClient.String()+"/32", "via", forServer.String())

			// 发送ip dispatch包, 然后开始转发数据包
			c.Write([]byte(forClient.String()))

			errorChan := make(chan error)
			tunReadChan := make(chan []byte)
			tunReadExitChan := make(chan struct{})
			connectionReadChan := make(chan []byte)
			connectionReadExitChan := make(chan struct{})

			// connection reader
			go func() {
				buf := make([]byte, 1500, 1500)
				for {
					c.SetReadDeadline(time.Now().Add(ReadTimeout))
					n, err := c.Read(buf)
					if err != nil {
						errorChan <- err
						<-connectionReadExitChan
						return
					}
					copyBuf := make([]byte, n, n)
					copy(copyBuf, buf)
					select {
					case connectionReadChan <- copyBuf:
					case <-connectionReadExitChan:
						return
					}
				}
			}()

			// tun reader
			go func() {
				buf := make([]byte, 1500, 1500)
				for {
					n, err := tun.Read(buf)
					if err != nil {
						<-tunReadChan
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

			for {
				select {
				case err := <-errorChan:
					log.Printf("error happened, closing the tun and connection: %v\n", err)
					tunReadExitChan <- struct{}{}
					connectionReadExitChan <- struct{}{}
					return
				case ipPacketBytes := <-tunReadChan:
					_, err := c.Write(ipPacketBytes)
					if err != nil {
						log.Printf("error happened, closing the tun and connection: %v\n", err)
						tunReadExitChan <- struct{}{}
						connectionReadExitChan <- struct{}{}
						return
					}
				case msg := <-connectionReadChan:
					if string(msg) == string(comm.HeartMagicPacket) {
						_, err := c.Write(msg)
						if err != nil {
							log.Printf("failed to write to tun: %v\n", err)
							tunReadExitChan <- struct{}{}
							connectionReadExitChan <- struct{}{}
							return
						}
					} else {
						_, err := tun.Write(msg)
						if err != nil {
							log.Printf("failed to write to tun: %v\n", err)
							tunReadExitChan <- struct{}{}
							connectionReadExitChan <- struct{}{}
							return
						}
					}
				}
			}
		}()
	}
}
