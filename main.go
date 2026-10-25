package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	fmt.Println("listening on 127.0.0.1:8080")

	conn, err := listener.Accept()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	request, err := readRequest(bufio.NewReader(conn))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("method:  %s\n", request.Method)
	fmt.Printf("target:  %s\n", request.Target)
	fmt.Printf("path:    %s\n", request.Path)
	fmt.Printf("query:   %s\n", request.Query)
	fmt.Printf("proto:   %s\n", request.Proto)
	fmt.Println("headers:")
	for name, value := range request.Headers {
		fmt.Printf("  %s: %s\n", name, value)
	}
}
