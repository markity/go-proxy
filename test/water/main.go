package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

var echoCmd string

func init() {
	var err error
	echoCmd, err = exec.LookPath("echo")
	if err != nil {
		panic(err)
	}
}

func MustEchoCmd(args ...string) {
	log.Println(echoCmd, strings.Join(args, " "))
	cmd := exec.Command(echoCmd, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to Run EchoCmd: %v\n", err)
	}
}

func main() {
	MustEchoCmd("$(ip route|grep default|cut -d' ' -f3)")
}
