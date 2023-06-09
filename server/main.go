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
			pwd, ok := UserMap[string(hint)]
			if !ok {
				return nil, errors.New("unregistered username")
			}
			return []byte(pwd), nil
		},
		CipherSuites:   []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_GCM_SHA256},
		MTU:            1500,
		ConnectTimeout: &comm.ConnectTimeout,
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
			forServerV4 := ipv4Poll.Next()
			forClientV4 := ipv4Poll.Next()
			defer func() {
				fmt.Println("connection closed")
				ipv4Poll.Release(forServerV4)
				ipv4Poll.Release(forClientV4)
			}()

			tun, err := water.New(water.Config{DeviceType: water.TUN})
			if err != nil {
				log.Fatalf("failed to create tun: %v\n", err)
			}

			comm.MustIPCmd("link", "set", tun.Name(), "up", "mtu", "1200")
			comm.MustIPCmd("addr", "add", forServerV4.String(), "dev", tun.Name())
			comm.MustIPCmd("route", "add", forClientV4.String()+"/32", "dev", tun.Name())

			// 发送ip dispatch包, 然后开始转发数据包
			c.Write([]byte(forClientV4.String()))

			heartbeatLostCount := 0

			tickChan := time.NewTicker(time.Second).C

			errorChan := make(chan error, 2)

			tunReaderChan := make(chan []byte)
			tunReaderExitChan := make(chan struct{})

			connectionReaderChan := make(chan []byte)
			connectionReaderExitChan := make(chan struct{})

			// connection reader
			go func() {
				buf := make([]byte, 1500)
				for {
					n, err := c.Read(buf)
					if err != nil {
						tun.Close()
						c.Close()
						errorChan <- err
						<-connectionReaderExitChan
						return
					}
					copyBuf := make([]byte, n)
					copy(copyBuf, buf)
					select {
					case connectionReaderChan <- copyBuf:
					case <-connectionReaderExitChan:
						return
					}
				}
			}()

			// tun reader
			go func() {
				buf := make([]byte, 1500)
				for {
					n, err := tun.Read(buf)
					if err != nil {
						tun.Close()
						c.Close()
						errorChan <- err
						<-tunReaderExitChan
						return
					}
					copyBuf := make([]byte, n)
					copy(copyBuf, buf[:n])
					select {
					case tunReaderChan <- copyBuf:
					case <-tunReaderExitChan:
						return
					}
				}
			}()

			for {
				select {
				case err := <-errorChan:
					tun.Close()
					c.Close()
					log.Printf("error happened, closing the tun and connection: %v\n", err)
					tunReaderExitChan <- struct{}{}
					connectionReaderExitChan <- struct{}{}
					return
				case ipPacketBytes := <-tunReaderChan:
					_, err := c.Write(ipPacketBytes)
					if err != nil {
						tun.Close()
						c.Close()
						log.Printf("error happened, closing the tun and connection: %v\n", err)
						tunReaderExitChan <- struct{}{}
						connectionReaderExitChan <- struct{}{}
						return
					}
				case msg := <-connectionReaderChan:
					if string(msg) == string(comm.HeartMagicPacket) {
						_, err := c.Write(msg)
						if err != nil {
							tun.Close()
							c.Close()
							log.Printf("failed to write to tun: %v\n", err)
							tunReaderExitChan <- struct{}{}
							connectionReaderExitChan <- struct{}{}
							return
						}
						heartbeatLostCount = 0
					} else {
						_, err := tun.Write(msg)
						if err != nil {
							tun.Close()
							c.Close()
							log.Printf("failed to write to tun: %v\n", err)
							tunReaderExitChan <- struct{}{}
							connectionReaderExitChan <- struct{}{}
							return
						}
					}
				case <-tickChan:
					heartbeatLostCount++
					if heartbeatLostCount > comm.MaxLostHeartbeatN {
						tun.Close()
						c.Close()
						log.Printf("max hearbeat lose reached, losing connection\n")
						tunReaderExitChan <- struct{}{}
						connectionReaderExitChan <- struct{}{}
						return
					}
				}
			}
		}()
	}
}
