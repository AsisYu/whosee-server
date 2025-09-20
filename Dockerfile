FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装依赖
RUN apk add --no-cache git

# 拷贝go.mod和go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 拷贝源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# 第二阶段：Chrome运行镜像
FROM alpine:latest

# 安装Chrome/Chromium及其依赖
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    chromium \
    chromium-chromedriver \
    nss \
    freetype \
    harfbuzz \
    font-noto-emoji \
    wqy-zenhei \
    # 中文字体支持
    && rm -rf /var/cache/apk/*

# 创建必要的目录
RUN mkdir -p /app/logs /app/static/screenshots /app/static/itdog \
    && mkdir -p /tmp/chrome-user-data \
    && chmod 755 /tmp/chrome-user-data

# 创建非root用户用于运行Chrome
RUN addgroup -g 1001 appgroup \
    && adduser -D -u 1001 -G appgroup appuser \
    && chown -R appuser:appgroup /app /tmp/chrome-user-data

# 设置工作目录
WORKDIR /app

# 从builder阶段复制编译好的应用
COPY --from=builder /app/main /app/

# 复制静态资源和配置文件
COPY --from=builder /app/static /app/static/
COPY --from=builder /app/.env.example /app/.env

# 复制Chrome运行时（如果存在）
COPY --from=builder /app/chrome_runtime /app/chrome_runtime

# 修改文件权限
RUN chown -R appuser:appgroup /app

# 暴露端口
EXPOSE 3900

# 设置环境变量 - Chrome优化
ENV GIN_MODE=release \
    PORT=3900 \
    REDIS_ADDR=redis:6379 \
    CHROME_BIN=/usr/bin/chromium-browser \
    CHROME_NO_SANDBOX=true \
    CHROME_DISABLE_GPU=true \
    CHROME_DISABLE_DEV_SHM=true \
    CHROME_USER_DATA_DIR=/tmp/chrome-user-data \
    CHROME_MODE=auto \
    # 内存和性能优化
    GOMEMLIMIT=800MiB \
    GOGC=100

# 切换到非root用户
USER appuser

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3900/api/health || exit 1

# 运行应用
CMD ["/app/main"]
