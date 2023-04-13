package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

var ipcmd string
var shcmd string
var ifconfigcmd string

func init() {
	var err error
	ipcmd, err = exec.LookPath("ip")
	if err != nil {
		panic(err)
	}

	shcmd, err = exec.LookPath("sh")
	if err != nil {
		panic(err)
	}

	ifconfigcmd, err = exec.LookPath("ifconfig")
	if err != nil {
		panic(err)
	}
}

func MustIPCmd(args ...string) {
	log.Println(ipcmd, strings.Join(args, " "))
	cmd := exec.Command(ipcmd, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to Run IPCmd: %v\n", err)
	}
}

func MustIfconfigCmd(args ...string) {
	log.Println(ifconfigcmd, strings.Join(args, " "))
	cmd := exec.Command(ipcmd, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to Run IfconfigCmd: %v\n", err)
	}
}

func MustShCmd(args ...string) {
	log.Println(shcmd, strings.Join(args, " "))
	cmd := exec.Command(shcmd, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Printf("failed to Run ShCmd: %v\n", err)
	}
}
