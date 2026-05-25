ARG BUILD_FROM=ghcr.io/hassio-addons/base:latest

FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o frisi .

FROM ${BUILD_FROM}
COPY --from=builder /build/frisi /usr/bin/frisi
COPY run.sh /run.sh
RUN chmod a+x /run.sh
CMD ["/run.sh"]
