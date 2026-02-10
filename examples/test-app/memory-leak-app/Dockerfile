FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY main.go .
RUN go mod init memory-leak-app && \
    go build -o memory-leak-app main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/memory-leak-app .

EXPOSE 8080

CMD ["./memory-leak-app"]
