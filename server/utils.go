package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ipcmd string
var ifconfigcmd string

func init() {
	var err error
	ipcmd, err = exec.LookPath("ip")
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

func iocopy(dst io.Writer, src io.Reader) error {
	var fn func(time.Duration) error
	if f, ok := dst.(interface{ SetWriteDeadline(time.Duration) error }); ok {
		fn = f.SetWriteDeadline
	}

	buf := make([]byte, 1500)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return err
		}

		if fn != nil {
			fn(5 * time.Second)
		}

		if _, err = dst.Write(buf[:n]); err != nil {
			return err
		}
	}
}
