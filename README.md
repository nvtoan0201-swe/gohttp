# Mini HTTP server in Go

Project này xây một HTTP/1.1 server nhỏ bằng Go để nhìn rõ các lớp cốt lõi của một web server. Server cố ý không dùng `net/http`; thư viện đó sẽ được dùng làm tư liệu so sánh sau khi hoàn thành bản mini.

## Luồng xử lý

```text
TCP listener
  -> Accept một connection
  -> đọc request line và headers
  -> đọc body theo Content-Length
  -> chọn handler theo method + path
  -> ghi status line, headers và body
  -> đóng connection
```

Mỗi connection được xử lý trong một goroutine. Bản mini xử lý một request trên mỗi connection và luôn trả `Connection: close`.

## Cấu trúc

- `main.go`: đăng ký các route mẫu và khởi động server.
- `request.go`: parse HTTP/1.1 request và áp dụng giới hạn kích thước.
- `response.go`: serialize `Response` thành bytes HTTP.
- `server.go`: listen, accept, goroutine, routing và ánh xạ lỗi giao thức.
- `server_test.go`: unit test parser/routing và integration test qua TCP.

## Chạy

```sh
go run .
```

Server mặc định ở `127.0.0.1:8080`. Có thể đổi địa chỉ:

```sh
ADDR=127.0.0.1:9090 go run .
```

Thử các endpoint:

```sh
curl -i http://127.0.0.1:8080/
curl -i 'http://127.0.0.1:8080/hello?name=Minh'
curl -i -X POST --data 'xin chào' http://127.0.0.1:8080/echo
```

Muốn xem request và response thật:

```sh
curl -v 'http://127.0.0.1:8080/hello?name=Minh'
```

## Kiểm tra

```sh
go test ./...
go vet ./...
```

## Giới hạn có chủ ý

Server chỉ hỗ trợ HTTP/1.1, route chính xác theo method + path, một request trên mỗi connection và body có `Content-Length`. Request line tối đa 8 KiB, headers tối đa 64 KiB và body tối đa 1 MiB.

Các phần chưa triển khai gồm HTTPS, keep-alive, chunked transfer encoding, HTTP/2, HTTP/3, middleware, static files và graceful shutdown. Chúng phù hợp cho các bước nâng cao sau khi đã hiểu luồng cốt lõi.
