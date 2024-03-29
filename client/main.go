package main

import (
	"flag"
	"fmt"
	"go-proxy/comm"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/pion/dtls"
	"github.com/songgao/water"
	"inet.af/netaddr"
)

var u *string
var p *string
var sigintChan chan os.Signal
var config *dtls.Config
var addr *net.UDPAddr

func init() {
	if os.Getuid() != 0 {
		println("Root is required to use this command")
		os.Exit(0)
	}

	u = flag.String("u", "", "username")
	p = flag.String("p", "", "password")
	flag.Parse()
	if *u == "" || *p == "" {
		log.Printf("usage: %v -u <username> -p <password>\n", os.Args[0])
		os.Exit(0)
	}

	sigintChan = make(chan os.Signal, 1)
	signal.Notify(sigintChan, os.Interrupt)

	config = &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			return []byte(*p), nil
		},
		PSKIdentityHint: []byte(*u),
		CipherSuites:    []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_GCM_SHA256},
		MTU:             1500,
		ConnectTimeout:  &comm.ConnectTimeout,
	}

	var err error
	addr, err = net.ResolveUDPAddr("udp", ServerIP+":"+fmt.Sprint(ServerPort))
	if err != nil {
		log.Printf("failed to ResolveUDPAddr: %v\n", err)
		os.Exit(0)
	}
}

func run() {
	doExit := false
	defer func() {
		if doExit {
			os.Exit(0)
		}
	}()

	log.Println("dialing to:", addr)
	n1 := time.Now()
	c, err := dtls.Dial("udp", addr, config)
	if err != nil {
		log.Printf("failed to dial to server: %v\n", err)
		return
	}
	defer c.Close()
	n2 := time.Now()

	// 读取ip dispatch包
	packetBytes := make([]byte, 1500)
	c.SetReadDeadline(time.Now().Add(comm.ConnectTimeout - n2.Sub(n1)))
	n, err := c.Read(packetBytes)
	if err != nil {
		log.Printf("failed to read: %v\n", err)
		return
	}

	c.SetReadDeadline(time.Time{})

	// 读到的第一个数据包应该是ip dispatch包
	ipString := string(packetBytes[:n])
	ipForClientV4, err := netaddr.ParseIP(ipString)
	if err != nil || !ipForClientV4.Is4() {
		log.Printf("protocol error: unexpected ip dispatch packet read")
		return
	}

	log.Printf("succeed to fetch ip: %v\n", ipForClientV4)

	// 开tun
	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Printf("failed to create tun device: %v\n", err)
		return
	}
	defer tun.Close()

	comm.MustIPCmd("link", "set", tun.Name(), "up", "mtu", "1200")
	comm.MustIPCmd("addr", "add", ipString, "dev", tun.Name())

	log.Println("creating route table and dns server...")

	var defaultGateway = comm.MustShCmdGetOutput("-c", "ip route|grep default|cut -d' ' -f3")
	if strings.ContainsRune(defaultGateway, '\n') {
		defaultGateway = defaultGateway[0:strings.IndexRune(defaultGateway, '\n')]
	}

	// 通过脚本执行
	shFmt := `
ip route add 0.0.0.0/1 dev %v
ip route add 128.0.0.0/1 dev %v
ip route add %v via %v`
	comm.MustShCmd("-c", fmt.Sprintf(shFmt, tun.Name(), tun.Name(), ServerIP, defaultGateway))
	defer func() {
		shFmt = `ip route del %v via %v`
		comm.MustShCmd("-c", fmt.Sprintf(shFmt, ServerIP, defaultGateway))
	}()

	originFileData, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		log.Printf("failed to read dns server file: %v\n", err)
		return
	}

	f, err := os.OpenFile("/etc/resolv.conf.head", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0)
	if err != nil {
		log.Printf("failed to edit dns server file: %v\n", err)
		return
	}
	defer func() {
		f.Close()
		err := os.Remove("/etc/resolv.conf.head")
		if err != nil {
			fmt.Println("failed to remove /etc/resolv.conf.head file")
		}
		resolvConfFile, err := os.OpenFile("/etc/resolv.conf", os.O_TRUNC|os.O_WRONLY, 0)
		if err != nil {
			fmt.Println("failed to recover /etc/resolv.conf file")
		}
		resolvConfFile.WriteString(string(originFileData))
		resolvConfFile.Close()
	}()

	dnsServerFileData := ""
	for _, v := range DNSServerIPS {
		dnsServerFileData += "nameserver " + v + "\n"
	}

	_, err = f.WriteString(dnsServerFileData)
	if err != nil {
		log.Printf("failed to edit dns server file: %v\n", err)
		return
	}

	log.Printf("transferring data...\n")

	heartbeatLostCount := 0

	tickerChan := time.NewTicker(comm.HeartbeatInterval).C

	errorChan := make(chan error, 2)

	tunReaderChan := make(chan []byte)
	tunReaderExitChan := make(chan struct{})

	connectionReaderChan := make(chan []byte)
	connectionReaderExitChan := make(chan struct{})

	// tun reader
	go func() {
		buf := make([]byte, 1500)
		for {
			n, err := tun.Read(buf)
			if err != nil {
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

	// connection reader
	go func() {
		buf := make([]byte, 1500)
		for {
			n, err := c.Read(buf)
			if err != nil {
				errorChan <- err
				<-connectionReaderExitChan
				return
			}

			copyBuf := make([]byte, n)
			copy(copyBuf, buf[:n])
			select {
			case connectionReaderChan <- copyBuf:
			case <-connectionReaderExitChan:
				return
			}
		}
	}()

	for {
		select {
		case <-tickerChan:
			_, err := c.Write(comm.HeartMagicPacket)
			if err != nil {
				tun.Close()
				c.Close()
				tunReaderExitChan <- struct{}{}
				connectionReaderExitChan <- struct{}{}
				log.Printf("failed to write to connection: %v\n", err)
				return
			}
			heartbeatLostCount++
			if heartbeatLostCount > comm.MaxLostHeartbeatN {
				tun.Close()
				c.Close()
				tunReaderExitChan <- struct{}{}
				connectionReaderExitChan <- struct{}{}
				log.Printf("max hearbeat lose reached, losing connection\n")
				return
			}
		case err := <-errorChan:
			tun.Close()
			c.Close()
			tunReaderExitChan <- struct{}{}
			connectionReaderExitChan <- struct{}{}
			log.Printf("error happened: %v\n", err)
			return
		case ipPacketContent := <-tunReaderChan:
			_, err := c.Write(ipPacketContent)
			if err != nil {
				tun.Close()
				c.Close()
				tunReaderExitChan <- struct{}{}
				connectionReaderExitChan <- struct{}{}
				log.Printf("failed to write to connection: %v\n", err)
				return
			}
		case msg := <-connectionReaderChan:
			if string(msg) == string(comm.HeartMagicPacket) {
				heartbeatLostCount = 0
				log.Printf("heartbeat received...\n")
			} else {
				_, err := tun.Write(msg)
				if err != nil {
					tun.Close()
					c.Close()
					tunReaderExitChan <- struct{}{}
					connectionReaderExitChan <- struct{}{}
					log.Printf("failed to write to tun: %v\n", err)
					return
				}
			}
		case <-sigintChan:
			tun.Close()
			c.Close()
			log.Printf("closing client\n")
			tunReaderExitChan <- struct{}{}
			connectionReaderExitChan <- struct{}{}
			doExit = true
			return
		}
	}
}

func main() {
	for {
		select {
		case <-sigintChan:
			return
		default:
			println("reconnecting")
			run()
		}
	}
}
