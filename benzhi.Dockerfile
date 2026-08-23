# 评测构建（与 Dockerfile 同源，供 build_benzhi_docker.sh 引用）
FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/microfluidbench ./cmd/microfluidbench

ENTRYPOINT ["/app/microfluidbench"]
CMD ["--smoke-test"]
