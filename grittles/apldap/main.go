// apldap is a Debug Adapter Protocol (DAP) server for Dyalog APL.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/cursork/gritt/dap/adapter"
)

func main() {
	// Command line flags
	port := flag.Int("port", 0, "TCP port to listen on (0 for stdio mode)")
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("apldap v0.1.0 - Debug Adapter Protocol server for Dyalog APL")
		os.Exit(0)
	}

	if *port > 0 {
		// Server mode - listen on TCP port
		runServer(*port)
	} else {
		// Stdio mode - communicate via stdin/stdout
		runStdio()
	}
}

func runStdio() {
	// Log to a file for debugging
	f, _ := os.OpenFile("/tmp/apldap.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if f != nil {
		log.SetOutput(f)
	} else {
		log.SetOutput(os.Stderr)
	}
	log.Println("APL DAP adapter starting in stdio mode")

	a := adapter.New(os.Stdin, os.Stdout)
	if err := a.Run(); err != nil {
		log.Fatalf("Adapter error: %v", err)
	}
}

func runServer(port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	defer listener.Close()

	log.Printf("APL DAP adapter listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("New connection from %s", conn.RemoteAddr())

	a := adapter.New(conn, conn)
	if err := a.Run(); err != nil {
		log.Printf("Session error: %v", err)
	}
	log.Printf("Connection closed: %s", conn.RemoteAddr())
}
