FROM golang:1.26-bookworm

ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct

RUN apt-get update && apt-get install -y python3-pip python3-requests && \
    python3 -m pip install cloudscraper --break-system-packages || true

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-w -s" -o /app/go-alpha
# RUN go build -o /app/go-alpha

EXPOSE 8080
ENTRYPOINT [ "/app/go-alpha" ]
