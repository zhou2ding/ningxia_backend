# 构建阶段
FROM --platform=linux/amd64 golang:1.22-alpine AS builder

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译Go程序
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o backend .

# 最终运行环境
FROM --platform=linux/amd64 python:3.9-slim

WORKDIR /app

# 安装Python依赖
RUN pip install --no-cache-dir python-docx

# 从构建阶段复制文件，会自动创建不存在的目录
COPY --from=builder /app/backend .
COPY --from=builder /app/road.yaml .
#COPY --from=builder /app/pys/process.py ./pys

# 暴露端口
EXPOSE 12345

# 启动命令
CMD ["./backend"]