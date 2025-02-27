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

# 从构建阶段复制文件
COPY --from=builder /app/backend /app/backend
COPY --from=builder /app/process.py /app/process.py
COPY --from=builder /app/template.docx /app/template.docx
COPY --from=builder /app/road.yaml /app/road.yaml

# 创建上传目录
RUN mkdir -p ./tmp/uploads && chmod 755 ./tmp/uploads

# 暴露端口
EXPOSE 12345

# 启动命令
CMD ["./backend"]