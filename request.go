package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxRequestLineBytes = 8 << 10
	maxHeaderBytes      = 64 << 10
	maxBodyBytes        = 1 << 20
)

var (
	errMalformedRequest            = errors.New("malformed request")
	errRequestLineTooLarge         = errors.New("request line too large")
	errHeaderTooLarge              = errors.New("request headers too large")
	errBodyTooLarge                = errors.New("request body too large")
	errUnsupportedTransferEncoding = errors.New("transfer-encoding is not supported")
	errUnsupportedHTTPVersion      = errors.New("http version is not supported")
)

type Request struct {
	Method  string
	Target  string
	Proto   string
	Path    string
	Query   string
	Headers map[string]string
	Body    []byte
}

func (r *Request) Header(name string) string {
	return r.Headers[strings.ToLower(name)]
}

func (r *Request) QueryValue(name string) string {
	values, err := url.ParseQuery(r.Query)
	if err != nil {
		return ""
	}
	return values.Get(name)
}

// readRequest parses one HTTP/1.1 request. This mini server only supports
// bodies whose length is declared by Content-Length.
func readRequest(reader *bufio.Reader) (*Request, error) {
	requestLine, err := readHTTPLine(reader, maxRequestLineBytes, errRequestLineTooLarge)
	if err != nil {
		return nil, err
	}

	parts := strings.Fields(requestLine)
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: request line must contain method, target, and version", errMalformedRequest)
	}
	if parts[2] != "HTTP/1.1" {
		return nil, fmt.Errorf("%w: %s", errUnsupportedHTTPVersion, parts[2])
	}

	target, err := url.ParseRequestURI(parts[1])
	if err != nil || target.Path == "" {
		return nil, fmt.Errorf("%w: invalid request target", errMalformedRequest)
	}

	headers := make(map[string]string)
	headerBytes := 0
	for {
		line, err := readHTTPLine(reader, maxRequestLineBytes, errHeaderTooLarge)
		if err != nil {
			return nil, err
		}
		headerBytes += len(line) + 2
		if headerBytes > maxHeaderBytes {
			return nil, errHeaderTooLarge
		}
		if line == "" {
			break
		}

		rawName, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(rawName) == "" {
			return nil, fmt.Errorf("%w: invalid header", errMalformedRequest)
		}
		name := strings.ToLower(strings.TrimSpace(rawName))
		if _, duplicate := headers[name]; duplicate &&
			(name == "content-length" || name == "transfer-encoding") {
			return nil, fmt.Errorf("%w: duplicate %s header", errMalformedRequest, name)
		}
		headers[name] = strings.TrimSpace(value)
	}

	if headers["host"] == "" {
		return nil, fmt.Errorf("%w: HTTP/1.1 requires a Host header", errMalformedRequest)
	}
	if _, present := headers["transfer-encoding"]; present {
		return nil, errUnsupportedTransferEncoding
	}

	var body []byte
	if rawLength, present := headers["content-length"]; present {
		length, err := strconv.ParseInt(rawLength, 10, 64)
		if err != nil || length < 0 {
			return nil, fmt.Errorf("%w: invalid content-length", errMalformedRequest)
		}
		if length > maxBodyBytes {
			return nil, errBodyTooLarge
		}

		body = make([]byte, length)
		if _, err := io.ReadFull(reader, body); err != nil {
			return nil, fmt.Errorf("%w: incomplete body", errMalformedRequest)
		}
	}

	return &Request{
		Method:  parts[0],
		Target:  parts[1],
		Proto:   parts[2],
		Path:    target.Path,
		Query:   target.RawQuery,
		Headers: headers,
		Body:    body,
	}, nil
}

func readHTTPLine(reader *bufio.Reader, limit int, tooLargeErr error) (string, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > limit {
			return "", tooLargeErr
		}
		line = append(line, part...)
		if err == nil {
			break
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return "", fmt.Errorf("%w: read line: %v", errMalformedRequest, err)
	}

	if !strings.HasSuffix(string(line), "\r\n") {
		return "", fmt.Errorf("%w: lines must end with CRLF", errMalformedRequest)
	}
	return string(line[:len(line)-2]), nil
}
