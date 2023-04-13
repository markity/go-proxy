package main

import "net"

func main() {
	net.ListenTCP("tcp", "8000")
}
