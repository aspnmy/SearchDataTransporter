
# 构建阶段
FROM golang:1.21.6-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache \
    gcc \
    musl-dev \
    linux-headers \
    git \
    pkgconfig \
    librdkafka-dev

# 设置工作目录
WORKDIR /build

# 复制源码
COPY . .

# 编译 Server 和 Client
RUN cd SearchClient && CGO_ENABLED=1 go build -o ../ollama_search_client && \
    cd .. && CGO_ENABLED=1 go build -o search_data_server

# 运行阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk add --no-cache \
    librdkafka \
    ca-certificates \
    tzdata

# 设置时区
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 设置工作目录
WORKDIR /app

# 复制编译产物
COPY --from=builder /build/search_data_server .
COPY --from=builder /build/ollama_search_client .

# 创建配置目录
RUN mkdir -p config

# 创建非 root 用户
RUN adduser -D -u 1000 searchapp && \
    chown -R searchapp:searchapp /app

# 切换到非 root 用户
USER searchapp

# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV KAFKA_BROKERS=localhost:9092 \
    HTTP_PORT=8080 \
    MAX_RESPONSE_SIZE=10485760 \
    RATE_LIMIT=100 \
    BURST_LIMIT=200

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q --spider http://localhost:8080/health || exit 1

# 设置入口点
ENTRYPOINT ["/app/search_data_server"]
