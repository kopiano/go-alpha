FROM golang:1.26.4

ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct

RUN apt-get update && apt-get install -y \
    python3-pip python3-requests \
    build-essential pkg-config libwebp-dev && \
    python3 -m pip install cloudscraper --break-system-packages || true

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-w -s" -o /app/go-alpha
RUN go build -o /app/go-alpha

EXPOSE 8080
ENTRYPOINT ["/app/go-alpha"]
