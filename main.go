package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	logger := log.New(os.Stdout, "http: ", log.LstdFlags)
	server := NewServer(addr, logger)

	server.Handle("GET", "/", func(*Request) Response {
		return Response{
			Body: []byte("mini Go HTTP server\nTry GET /hello?name=Minh or POST /echo\n"),
		}
	})

	server.Handle("GET", "/hello", func(request *Request) Response {
		name := request.QueryValue("name")
		if name == "" {
			name = "world"
		}
		return Response{
			Body: []byte(fmt.Sprintf("Hello, %s!\n", name)),
		}
	})

	server.Handle("POST", "/echo", func(request *Request) Response {
		body := make([]byte, len(request.Body)+1)
		copy(body, request.Body)
		body[len(request.Body)] = '\n'
		return Response{
			Status: 201,
			Body:   body,
		}
	})

	logger.Printf("listening on %s", addr)
	if err := server.ListenAndServe(); err != nil {
		logger.Fatal(err)
	}
}
