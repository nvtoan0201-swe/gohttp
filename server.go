package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"strings"
	"time"
)

type Handler func(*Request) Response

type Server struct {
	Addr    string
	Logger  *log.Logger
	routes  map[string]Handler
	methods map[string][]string
	timeout time.Duration
}

func NewServer(addr string, logger *log.Logger) *Server {
	return &Server{
		Addr:    addr,
		Logger:  logger,
		routes:  make(map[string]Handler),
		methods: make(map[string][]string),
		timeout: 5 * time.Second,
	}
}

func (s *Server) Handle(method, path string, handler Handler) {
	if method == "" || !strings.HasPrefix(path, "/") || handler == nil {
		panic("invalid route")
	}

	key := routeKey(method, path)
	if _, exists := s.routes[key]; exists {
		panic("duplicate route: " + key)
	}

	s.routes[key] = handler
	s.methods[path] = append(s.methods[path], method)
}

func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	return s.Serve(listener)
}

func (s *Server) Serve(listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept connection: %w", err)
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.timeout))

	request, err := readRequest(bufio.NewReaderSize(conn, 4<<10))
	if err != nil {
		status := statusForRequestError(err)
		if s.Logger != nil {
			s.Logger.Printf("request from %s rejected: %v", conn.RemoteAddr(), err)
		}
		_ = writeResponse(conn, Response{
			Status: status,
			Body:   []byte(statusText(status) + "\n"),
		})
		return
	}

	response := s.route(request)
	if err := writeResponse(conn, response); err != nil && s.Logger != nil {
		s.Logger.Printf("write response to %s: %v", conn.RemoteAddr(), err)
	}
}

func (s *Server) route(request *Request) Response {
	if handler, ok := s.routes[routeKey(request.Method, request.Path)]; ok {
		return handler(request)
	}

	methods, pathExists := s.methods[request.Path]
	if pathExists {
		methods = slices.Clone(methods)
		slices.Sort(methods)
		return Response{
			Status: 405,
			Headers: map[string]string{
				"Allow": strings.Join(methods, ", "),
			},
			Body: []byte("Method Not Allowed\n"),
		}
	}

	return Response{
		Status: 404,
		Body:   []byte("Not Found\n"),
	}
}

func statusForRequestError(err error) int {
	switch {
	case errors.Is(err, errBodyTooLarge):
		return 413
	case errors.Is(err, errRequestLineTooLarge), errors.Is(err, errHeaderTooLarge):
		return 431
	case errors.Is(err, errUnsupportedTransferEncoding):
		return 501
	case errors.Is(err, errUnsupportedHTTPVersion):
		return 505
	default:
		return 400
	}
}

func routeKey(method, path string) string {
	return method + " " + path
}
