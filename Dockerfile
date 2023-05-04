FROM golang:1.20 AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o beehive-nodes-service

FROM scratch
COPY --from=builder /build/beehive-nodes-service /beehive-nodes-service
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT [ "/beehive-nodes-service" ]
