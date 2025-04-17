FROM golang:1.23.1-alpine AS builder

RUN apk add --no-cache \
    build-base \
    linux-headers \
    iptables \
    bash \
    tcpdump \
    tshark \
    iproute2

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o /go/bin/tcpcustom ./cmd/tcpcustom

FROM alpine:3.18

RUN apk add --no-cache \
    iptables \
    bash \
    tcpdump \
    tshark \
    iproute2 \
    libcap

RUN mkdir -p /dev/net && \
    mknod /dev/net/tun c 10 200 && \
    chmod 600 /dev/net/tun

COPY --from=builder /go/bin/tcpcustom /usr/local/bin/tcpcustom

COPY scripts/* /usr/local/bin/
RUN chmod +x /usr/local/bin/setup.sh /usr/local/bin/iptables-rules.sh /usr/local/bin/capture-traffic.sh

RUN mkdir -p /root/captures && chmod 777 /root/captures

WORKDIR /root

ENTRYPOINT ["tcpcustom"]