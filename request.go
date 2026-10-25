package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var errMalformedRequest = errors.New("malformed request")

type Request struct {
	Method  string
	Target  string
	Proto   string
	Path    string
	Query   string
	Headers map[string]string
}

func (r *Request) Header(name string) string {
	return r.Headers[strings.ToLower(name)]
}

// readRequest parses the request line and headers. The body is left unread
// in the bufio.Reader for the next step.
func readRequest(reader *bufio.Reader) (*Request, error) {
	requestLine, err := readHTTPLine(reader)
	if err != nil {
		return nil, err
	}

	parts := strings.Fields(requestLine)
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: request line must contain method, target, and version", errMalformedRequest)
	}

	target, err := url.ParseRequestURI(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid request target", errMalformedRequest)
	}
	if target.Path == "" {
		target.Path = "/"
	}

	headers := make(map[string]string)
	for {
		line, err := readHTTPLine(reader)
		if err != nil {
			return nil, err
		}
		if line == "" {
			break
		}

		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("%w: invalid header", errMalformedRequest)
		}
		headers[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
	}

	return &Request{
		Method:  parts[0],
		Target:  parts[1],
		Proto:   parts[2],
		Path:    target.Path,
		Query:   target.RawQuery,
		Headers: headers,
	}, nil
}

func readHTTPLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("%w: read line: %v", errMalformedRequest, err)
	}
	if !strings.HasSuffix(line, "\r\n") {
		return "", fmt.Errorf("%w: lines must end with CRLF", errMalformedRequest)
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}
