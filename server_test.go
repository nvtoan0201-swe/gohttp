package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestReadRequest(t *testing.T) {
	raw := "POST /echo?from=test HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"hello"

	request, err := readRequest(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatal(err)
	}

	if request.Method != "POST" || request.Path != "/echo" || request.Query != "from=test" {
		t.Fatalf("unexpected request line: %#v", request)
	}
	if request.Proto != "HTTP/1.1" {
		t.Fatalf("Proto = %q, want HTTP/1.1", request.Proto)
	}
	if got := request.Header("content-type"); got != "text/plain" {
		t.Fatalf("Header() = %q, want text/plain", got)
	}
	if request.QueryValue("from") != "test" {
		t.Fatalf("QueryValue(from) = %q, want test", request.QueryValue("from"))
	}
	if string(request.Body) != "hello" {
		t.Fatalf("Body = %q, want hello", request.Body)
	}
}

func TestReadRequestRejectsUnsupportedBodiesAndVersions(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want error
	}{
		{
			name: "chunked body",
			raw:  "POST /echo HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n",
			want: errUnsupportedTransferEncoding,
		},
		{
			name: "http version",
			raw:  "GET / HTTP/1.0\r\nHost: example.com\r\n\r\n",
			want: errUnsupportedHTTPVersion,
		},
		{
			name: "missing host",
			raw:  "GET / HTTP/1.1\r\n\r\n",
			want: errMalformedRequest,
		},
		{
			name: "invalid content length",
			raw:  "POST /echo HTTP/1.1\r\nHost: example.com\r\nContent-Length: nope\r\n\r\n",
			want: errMalformedRequest,
		},
		{
			name: "body too large",
			raw: fmt.Sprintf(
				"POST /echo HTTP/1.1\r\nHost: example.com\r\nContent-Length: %d\r\n\r\n",
				maxBodyBytes+1,
			),
			want: errBodyTooLarge,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := readRequest(bufio.NewReader(strings.NewReader(test.raw)))
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestReadRequestRejectsOversizedRequestLine(t *testing.T) {
	raw := "GET /" + strings.Repeat("a", maxRequestLineBytes) + " HTTP/1.1\r\nHost: example.com\r\n\r\n"

	_, err := readRequest(bufio.NewReader(strings.NewReader(raw)))
	if !errors.Is(err, errRequestLineTooLarge) {
		t.Fatalf("error = %v, want request line too large", err)
	}
}

func TestWriteResponse(t *testing.T) {
	var wire bytes.Buffer
	err := writeResponse(&wire, Response{
		Status: 201,
		Headers: map[string]string{
			"X-Test": "yes",
		},
		Body: []byte("created"),
	})
	if err != nil {
		t.Fatal(err)
	}

	response, err := http.ReadResponse(bufio.NewReader(&wire), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 201 {
		t.Fatalf("StatusCode = %d, want 201", response.StatusCode)
	}
	if response.Header.Get("X-Test") != "yes" {
		t.Fatalf("X-Test = %q, want yes", response.Header.Get("X-Test"))
	}
	if response.ContentLength != int64(len("created")) || string(body) != "created" {
		t.Fatalf("unexpected response body or length: %d, %q", response.ContentLength, body)
	}
}

func TestRouterReturnsNotFoundAndMethodNotAllowed(t *testing.T) {
	server := NewServer("", nil)
	server.Handle("GET", "/items", func(*Request) Response {
		return Response{Body: []byte("items")}
	})
	server.Handle("POST", "/items", func(*Request) Response {
		return Response{Status: 201}
	})

	missing := server.route(&Request{Method: "GET", Path: "/missing"})
	if missing.Status != 404 {
		t.Fatalf("missing route status = %d, want 404", missing.Status)
	}

	wrongMethod := server.route(&Request{Method: "DELETE", Path: "/items"})
	if wrongMethod.Status != 405 {
		t.Fatalf("wrong method status = %d, want 405", wrongMethod.Status)
	}
	if allow := wrongMethod.Headers["Allow"]; allow != "GET, POST" {
		t.Fatalf("Allow = %q, want GET, POST", allow)
	}
}

func TestServerHandlesConcurrentTCPRequests(t *testing.T) {
	server := NewServer("", nil)
	server.Handle("POST", "/echo", func(request *Request) Response {
		return Response{Status: 201, Body: request.Body}
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(listener)
	}()

	const clientCount = 4
	var clients sync.WaitGroup
	clientErrors := make(chan error, clientCount)
	for index := range clientCount {
		clients.Add(1)
		go func() {
			defer clients.Done()
			clientErrors <- sendEchoRequest(listener.Addr().String(), fmt.Sprintf("client-%d", index))
		}()
	}
	clients.Wait()
	close(clientErrors)

	for err := range clientErrors {
		if err != nil {
			t.Error(err)
		}
	}

	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-serveDone; err != nil {
		t.Fatalf("Serve() error: %v", err)
	}
}

func sendEchoRequest(addr, message string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = fmt.Fprintf(
		conn,
		"POST /echo HTTP/1.1\r\nHost: localhost\r\nContent-Length: %d\r\n\r\n%s",
		len(message),
		message,
	)
	if err != nil {
		return err
	}

	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != 201 || string(body) != message {
		return fmt.Errorf("response = %d %q, want 201 %q", response.StatusCode, body, message)
	}
	return nil
}
