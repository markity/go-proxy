package main

import (
	"errors"
	"go-proxy/comm"
	"log"
	"net"
	"time"

	"github.com/pion/dtls"
	"github.com/songgao/water"
)

var lastHint string

func main() {
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			hintStr := string(hint)
			pwd, ok := UserMap[hintStr]
			if !ok {
				return nil, errors.New("unregistered username")
			}
			lastHint = string(hint)
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
			log.Printf("Accept error: %v\n", err)
			continue
		}

		loginExitChan := make(chan struct{}, 1)
		if last := LoginSession[lastHint]; last != nil {
			last <- struct{}{}
		}
		LoginSession[lastHint] = loginExitChan

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

			comm.MustIPCmd("link", "set", tun.Name(), "up", "mtu", "1300")
			comm.MustIPCmd("addr", "add", forServer.String(), "dev", tun.Name())
			comm.MustIPCmd("route", "add", forClient.String()+"/32", "via", forServer.String())

			// 发送ip dispatch包, 然后开始转发数据包
			c.Write(comm.NewIPDispatchPacket(forClient, forServer).Pack())

			errorChan := make(chan error)
			tunReadChan := make(chan []byte)
			tunReadExitChan := make(chan struct{})
			connectionReadChan := make(chan interface{})
			connectionReadExitChan := make(chan struct{})

			// connection reader
			go func() {
				buf := make([]byte, 1400, 1400)
				for {
					c.SetReadDeadline(time.Now().Add(ReadTimeout))
					n, err := c.Read(buf)
					if err != nil {
						errorChan <- err
						<-connectionReadExitChan
						return
					}
					select {
					case connectionReadChan <- comm.ParsePacket(buf[:n]):
					case <-connectionReadExitChan:
						return
					}
				}
			}()

			// tun reader
			go func() {
				buf := make([]byte, 1300, 1300)
				for {
					n, err := tun.Read(buf)
					if err != nil {
						errorChan <- err
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

			select {
			case err := <-errorChan:
				log.Printf("error happened, closing the tun and connection: %v\n", err)
				tunReadExitChan <- struct{}{}
				connectionReadExitChan <- struct{}{}
				return
			case ipPacketBytes := <-tunReadChan:
				_, err := c.Write(comm.NewIPPacketPacket(string(ipPacketBytes)).Pack())
				if err != nil {
					log.Printf("error happened, closing the tun and connection: %v\n", err)
					tunReadExitChan <- struct{}{}
					connectionReadExitChan <- struct{}{}
					return
				}
			case msg := <-connectionReadChan:
				switch v := msg.(type) {
				case *comm.IPPacketPacket:
					_, err := tun.Write([]byte(v.Data))
					if err != nil {
						log.Printf("failed to write to tun: %v\n", err)
						tunReadExitChan <- struct{}{}
						connectionReadExitChan <- struct{}{}
						return
					}
				case *comm.HeartPacket:
					_, err := c.Write(comm.NewHeartPacket().Pack())
					if err != nil {
						log.Printf("failed to write to connection, closing the tun and connection: %v\n", err)
						tunReadExitChan <- struct{}{}
						connectionReadExitChan <- struct{}{}
						return
					}
				default:
					log.Printf("protocol error, closing tun and connection: unexpected packet received\n")
					tunReadExitChan <- struct{}{}
					connectionReadExitChan <- struct{}{}
					return
				}
			case <-loginExitChan:
				log.Printf("external exit, closing tun and connection")
				tunReadExitChan <- struct{}{}
				connectionReadExitChan <- struct{}{}
				return
			}
		}()
	}
}
