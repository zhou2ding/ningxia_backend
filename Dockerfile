# 构建阶段
FROM --platform=linux/amd64 golang:1.22-bullseye AS builder

WORKDIR /app

# 安装 SQLite 开发库
RUN apt-get update && apt-get install -y sqlite3 libsqlite3-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 启用 CGO 并编译
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o backend .

# 最终运行环境
FROM --platform=linux/amd64 python:3.9-slim

WORKDIR /app

# 安装运行时依赖
RUN apt-get update && apt-get install -y libsqlite3-0

# 安装 Python 依赖
RUN pip install --no-cache-dir python-docx

# 复制必要的文件
COPY --from=builder /app/backend .
COPY --from=builder /app/road.yaml .

EXPOSE 12345

CMD ["./backend"]