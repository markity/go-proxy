package main

import (
	"encoding/json"
	"fmt"
	"go-proxy/comm"
	"log"
	"net"
	"strings"
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

	ipv4_poll := NewAddrPool("10.0.0.0/24")

	gw_ipv4 := ipv4_poll.NextPrefix()

	for {
		// accept
		c, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error: %v\n", err)
			continue
		}

		go func() {
			target_v4 := ipv4_poll.NextPrefix()
			defer func() {
				ipv4_poll.Release(target_v4.IP())
				c.Close()
			}()

			Mutex.Lock()
			if Logined {
				Mutex.Unlock()
				c.Close()
				return
			} else {
				Logined = true
				Mutex.Unlock()
				defer func() {
					Mutex.Lock()
					Logined = false
					Mutex.Unlock()
				}()
			}

			// 对udp不考虑write超时的情况
			fmt.Println(comm.NewIPDispatchPacket(target_v4.IP(), gw_ipv4.IP()).Pack())
			_, err := c.Write(comm.NewIPDispatchPacket(target_v4.IP(), gw_ipv4.IP()).Pack())
			if err != nil {
				// unreachable
				panic(err)
			}

			tun, err := water.New(water.Config{DeviceType: water.TUN})
			if err != nil {
				log.Fatalf("failed to create tun: %v\n", err)
			}
			defer tun.Close()

			MustIPCmd("link", "set", tun.Name(), "up", "mtu", "1300")
			MustIPCmd("addr", "add", gw_ipv4.String(), "peer", target_v4.String(), "dev", tun.Name())
			MustIPCmd("route", "add", strings.Split(c.RemoteAddr().String(), ":")[0]+"/32", "via", target_v4.IP().String())

			var errStringChan = make(chan string)
			var connectionMessageChan = make(chan interface{})
			var tunChan = make(chan []byte)

			// conn reader
			go func() {
				buf := make([]byte, 1400, 1400)
				// 3s收不到包就关了
				for {
					c.SetReadDeadline(time.Now().Add(time.Second * 3))
					_, err := c.Read(buf)
					if err != nil {
						errStringChan <- err.Error()
						return
					}

					switch comm.ParsePacket(buf) {
					case comm.HeartPacketType:
						connectionMessageChan <- comm.NewHeartPacket()
					case comm.IPDispatchPacketType:
						var p comm.IPDispatchPacket
						json.Unmarshal(buf, &p)
						connectionMessageChan <- &p
					case comm.IPPacketPacketType:
						var p comm.IPPacketPacket
						json.Unmarshal(buf, &p)
						connectionMessageChan <- &p
					}
				}
			}()

			// tun reader
			go func() {
				for {
					buf := make([]byte, 1400, 1400)
					n, err := tun.Read(buf)
					if err != nil {
						// 这里出现错误的原因是外部把tun关了, 我们需要简单地退出就行了
						return
					}
					tunChan <- buf[:n]
				}
			}()

			select {
			case m := <-errStringChan:
				log.Printf("connection read error, now close the tunnel: %v\n", m)
				return
			case m := <-connectionMessageChan:
				switch v := m.(type) {
				case *comm.HeartPacket:
					// 维护连接状态的是read, 如果read三秒没有收到包, 就放弃与对方通讯
					c.Write(v.Pack())
				case *comm.IPPacketPacket:
					tun.Write([]byte(v.Data))
				}
			case m := <-tunChan:
				_, err := c.Write(m)
				if err != nil {
					// unreachable
					panic(err)
				}
			}
		}()
	}
}
