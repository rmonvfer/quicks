package main

import (
	"fmt"
	"net"
)

func ListenAndServe(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("accept error: %v\n", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Printf("client connected: %s\n", conn.RemoteAddr())

	// read 1024 bytes indefinitely
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("read error: %v\n", err)
			return
		}
		fmt.Printf("received %d bytes:\n", n)
		fmt.Printf("raw: %q\n", buf[:n])

		// Reply with an empty array
		conn.Write([]byte("*0\r\n"))
	}
}
