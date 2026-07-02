FROM golang:1.26-bookworm

ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct

RUN apt-get update -o Acquire::AllowInsecureRepositories=yes -o Acquire::AllowDowngradeToInsecureRepositories=yes --allow-releaseinfo-change 2>/dev/null || true && \
    apt-get install -y --allow-unauthenticated python3-pip 2>/dev/null || true && \
    python3 -m pip install requests cloudscraper --break-system-packages 2>/dev/null || true

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/go-alpha

EXPOSE 8080
ENTRYPOINT [ "/app/go-alpha" ]
