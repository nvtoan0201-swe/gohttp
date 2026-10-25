package main

import (
	"fmt"
	"io"
	"strconv"
)

type Response struct {
	Status  int
	Headers map[string]string
	Body    []byte
}

func writeResponse(writer io.Writer, response Response) error {
	if response.Status == 0 {
		response.Status = 200
	}

	headers := make(map[string]string, len(response.Headers)+3)
	for name, value := range response.Headers {
		headers[name] = value
	}
	if headers["Content-Type"] == "" {
		headers["Content-Type"] = "text/plain; charset=utf-8"
	}
	headers["Content-Length"] = strconv.Itoa(len(response.Body))
	headers["Connection"] = "close"

	if _, err := fmt.Fprintf(
		writer,
		"HTTP/1.1 %d %s\r\n",
		response.Status,
		statusText(response.Status),
	); err != nil {
		return err
	}
	for name, value := range headers {
		if _, err := fmt.Fprintf(writer, "%s: %s\r\n", name, value); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(writer, "\r\n"); err != nil {
		return err
	}
	_, err := writer.Write(response.Body)
	return err
}

func statusText(status int) string {
	switch status {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 400:
		return "Bad Request"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 413:
		return "Content Too Large"
	case 431:
		return "Request Header Fields Too Large"
	case 500:
		return "Internal Server Error"
	case 501:
		return "Not Implemented"
	case 505:
		return "HTTP Version Not Supported"
	default:
		return "Unknown Status"
	}
}
