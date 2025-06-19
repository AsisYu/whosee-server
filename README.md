# Whosee.me 域名信息查询服务

## 项目概述

Whosee.me 是一个高性能的域名信息查询和分析服务，提供快速、可靠的域名WHOIS信息查询、DNS记录查询、网站截图和性能测试功能。该服务集成了多个数据提供商API，实现了负载均衡、故障转移和智能缓存，并采用高并发优化架构，确保查询服务的高可用性和响应速度。

前端仓库地址：[https://github.com/AsisYu/whosee-whois](https://github.com/AsisYu/whosee-whois)

## 主要特性

### 基础功能

- **多提供商支持**：集成WhoisXML和WhoisFreaks等多个WHOIS数据提供商，提高数据准确性和服务可用性
- **智能缓存系统**：基于Redis的缓存系统，根据域名到期时间动态调整缓存时长
- **负载均衡**：智能选择最佳数据提供商，平衡API调用次数
- **故障转移**：自动检测API故障并切换到备用提供商

### 高级特性

- **全面的健康检查系统**：提供统一的健康检查API，监控多种服务状态
- **智能Chrome管理**：支持冷启动、热启动、智能混合三种模式，自动下载和平台检测
- **增强的截图功能**：智能网站截图与详细错误反馈，资源优化管理
- **高并发优化架构**：全面优化的高并发处理能力
- **完善的错误处理**：统一的API响应格式和详细的错误分类
- **安全防护**：包含CORS、请求验证等安全措施

## 技术架构

### 核心组件

- **Web框架**：基于Gin构建的高性能Web服务
- **缓存系统**：使用Redis进行数据缓存和分布式限流
- **服务管理**：服务容器模式管理多个服务组件
- **智能Chrome工具**：支持智能混合模式，自动平台检测和资源优化
- **智能工作池**：基于CPU核心数的动态工作池，高效处理并发请求
- **熔断保护**：防止系统在故障条件下过载的熔断器模式
- **中间件**：认证、日志、限流、错误处理等中间件

### 目录结构

```
.
├── handlers/       # 请求处理器和异步处理函数
├── middleware/     # 中间件组件
├── providers/      # WHOIS数据提供商实现
├── services/       # 核心业务逻辑和服务组件
├── routes/         # API路由定义
├── types/          # 数据类型定义
├── utils/          # 辅助函数和工具
├── logs/           # 日志文件
├── static/         # 静态资源（截图等）
├── .env            # 环境变量配置
└── main.go         # 应用入口
```

## 安装指南

### 前置条件

- 🔹 Go 1.24+
- 🔹 Redis 6.0+
- 🔹 WHOIS API账号（WhoisXML和/或WhoisFreaks）
- 🔹 Chrome/Chromium (用于网站截图功能，支持自动下载和智能平台检测)

### 安装步骤

1. 克隆仓库

```bash
git clone https://github.com/AsisYu/whosee-server.git
cd whosee-server
```

2. 安装依赖

```bash
go mod download
```

3. 配置环境变量

```bash
cp .env.example .env
# 编辑.env文件，填入API密钥和Redis配置
```

4. 运行服务

```bash
go run main.go
```

## 部署指南

### 使用Docker部署

1. 构建Docker镜像

```bash
# 在服务端目录下
docker build -t whosee-server .
```

2. 运行容器

```bash
docker run -d -p 3900:3900 --name whosee-server \
  -e WHOISFREAKS_API_KEY=your_api_key \
  -e WHOISXML_API_KEY=your_api_key \
  -e JWT_SECRET=your_jwt_secret \
  -e API_KEY=your_api_key \
  --restart unless-stopped \
  whosee-server
```

### 使用PM2部署（生产环境）

1. 安装PM2

```bash
npm install -g pm2
```

2. 编译Go应用

```bash
go build -o whosee-server main.go
```

3. 创建PM2配置文件 `ecosystem.config.js`

```javascript
module.exports = {
  apps: [{
    name: "whosee-server",
    script: "./whosee-server",
    env: {
      NODE_ENV: "production",
      PORT: 3900
    },
    log_date_format: "YYYY-MM-DD HH:mm:ss",
    out_file: "./logs/pm2_out.log",
    error_file: "./logs/pm2_error.log"
  }]
}
```

4. 启动服务

```bash
pm2 start ecosystem.config.js
```

5. 设置开机自启

```bash
pm2 startup
pm2 save
```

### Nginx配置（反向代理）

```nginx
server {
    listen 80;
    server_name api.whosee.me;

    location / {
        proxy_pass http://localhost:3900;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## API接口说明

### 主要API端点

| 端点 | 方法 | 说明 | 参数 |
|------|------|------|------|
| `/api/health` | GET | 健康检查API，返回所有服务的健康状态 | `detailed=true/false`: 是否返回详细状态 |
| `/api/v1/whois` | GET | WHOIS信息查询（通过查询参数） | `domain`: 要查询的域名 |
| `/api/v1/whois/:domain` | GET | WHOIS信息查询（通过路径参数） | `:domain`: 路径中的域名 |
| `/api/v1/rdap` | GET | RDAP协议查询（通过查询参数）🆕 | `domain`: 要查询的域名 |
| `/api/v1/rdap/:domain` | GET | RDAP协议查询（通过路径参数）🆕 | `:domain`: 路径中的域名 |
| `/api/v1/dns` | GET | DNS记录查询（通过查询参数） | `domain`: 要查询的域名 |
| `/api/v1/dns/:domain` | GET | DNS记录查询（通过路径参数） | `:domain`: 路径中的域名 |
| `/api/v1/screenshot` | GET | 网站截图服务（通过查询参数） | `domain`: 要截图的域名 |
| `/api/v1/screenshot/:domain` | GET | 网站截图服务（通过路径参数） | `:domain`: 路径中的域名 |
| `/api/v1/screenshot/base64/:domain` | GET | 返回Base64编码的网站截图 | `:domain`: 路径中的域名 |
| `/api/v1/itdog/:domain` | GET | ITDog网站性能测试截图 | `:domain`: 路径中的域名 |
| `/api/v1/itdog/base64/:domain` | GET | 返回Base64编码的ITDog测试结果 | `:domain`: 路径中的域名 |
| `/api/auth/token` | POST | 获取安全访问令牌 | 无需参数，根据客户端IP发放短期令牌 |

### 安全认证

Whosee.me API采用JWT令牌认证机制确保API安全。当需要访问受保护的API端点时，需要先获取安全令牌。

#### 获取认证令牌

```bash
# 获取JWT令牌
curl -X POST http://localhost:3900/api/auth/token
```

响应示例:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MjE0MzI5NjQsImlhdCI6MTcyMTQzMjkzNCwiaXNzIjoid2hvaXMtYXBpLm9zLnRuIiwibm9uY2UiOiIxNzIxNDMyOTM0NTM0NjIxMzAwIiwiaXAiOiIxMjcuMC4wLjEifQ.X2mhJYGwOLmJQMYt4PqZFYKyHN7sN-F9_qZGpk1YdJE"
}
```

#### 使用令牌访问API

获取到令牌后，将其添加到请求头中进行认证：

```bash
# 使用认证令牌访问API
curl -X GET http://localhost:3900/api/v1/whois/example.com \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

注意：
- 令牌有效期为30秒
- 每个令牌只能使用一次
- 请求频率限制为每IP每分钟30个令牌

### 请求示例

#### WHOIS查询
```bash
# 通过查询参数
curl -X GET "http://localhost:3900/api/v1/whois?domain=example.com"

# 通过路径参数
curl -X GET http://localhost:3900/api/v1/whois/example.com
```

#### RDAP查询 🆕
```bash
# 通过查询参数（专用IANA-RDAP提供商）
curl -X GET "http://localhost:3900/api/v1/rdap?domain=example.com"

# 通过路径参数
curl -X GET http://localhost:3900/api/v1/rdap/example.com
```

RDAP (Registration Data Access Protocol) 提供：
- 标准化JSON格式响应
- 更好的国际化支持
- 增强的安全性和隐私保护
- RESTful API设计

#### DNS记录查询
```bash
# 通过查询参数
curl -X GET "http://localhost:3900/api/v1/dns?domain=example.com"

# 通过路径参数
curl -X GET http://localhost:3900/api/v1/dns/example.com
```

#### 网站截图
```bash
# 获取截图
curl -X GET http://localhost:3900/api/v1/screenshot/example.com

# 获取Base64编码的截图
curl -X GET http://localhost:3900/api/v1/screenshot/base64/example.com
```

#### 健康检查API
```bash
# 基本健康检查
curl -X GET http://localhost:3900/api/health

# 详细健康检查
curl -X GET "http://localhost:3900/api/health?detailed=true"
```

### 统一响应格式

所有API返回的JSON格式如下:

#### 成功响应
```json
{
  "success": true,
  "data": {
    // 具体的响应数据
  },
  "meta": {
    "timestamp": "2025-05-10T03:21:17Z",
    "processingTimeMs": 245,
    "cached": false
  }
}
```

#### 错误响应
```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "错误描述"
  },
  "meta": {
    "timestamp": "2025-05-10T03:21:17Z"
  }
}
```

### 常见错误代码

| 错误代码 | HTTP状态码 | 描述 |
|---------|----------|------|
| `MISSING_PARAMETER` | 400 | 缺少必要的参数 |
| `INVALID_DOMAIN` | 400 | 无效的域名格式 |
| `RATE_LIMITED` | 429 | 请求频率超过限制 |
| `SERVICE_BUSY` | 503 | 服务忙碌 |
| `TIMEOUT` | 504 | 请求处理超时 |
| `QUERY_ERROR` | 500 | 查询过程中发生错误 |
| `SCREENSHOT_ERROR` | 500 | 截图过程中发生错误 |
| `ITDOG_ERROR` | 500 | ITDog测试过程中发生错误 |

## Chrome智能管理

### 三种运行模式

Whosee.me 集成了智能Chrome管理系统，特别适合WHOIS服务（主要功能）+ 偶尔截图的使用场景：

| 模式 | 启动方式 | 资源占用 | 响应速度 | 适用场景 | 空闲管理 |
|------|----------|----------|----------|----------|----------|
| **冷启动** | 每次重新启动 | 最低 | 慢(2-3秒) | 极少使用截图 | 用完即关 |
| **热启动** | 预热保持运行 | 较高 | 最快(<100ms) | 频繁使用截图 | 10分钟自动关闭 |
| **智能混合** | 按需+智能复用 | 中等 | 适中 | **WHOIS主业务+偶尔截图** | 智能调整(1.5-6分钟) |

### 智能混合模式

默认采用智能混合模式，具有以下特性：

- **智能启动策略**：首次使用快速启动，频繁使用自动切换为热启动策略
- **智能空闲管理**：根据使用频率动态调整空闲超时时间（1.5-6分钟）
- **实例复用**：健康的Chrome实例直接复用，避免重复启动开销
- **自动下载**：智能检测平台并自动下载Chrome，支持中国镜像源
- **并发控制**：最大3个并发操作，避免资源竞争

### Chrome配置

可通过环境变量 `CHROME_MODE` 配置运行模式：

```bash
# 设置Chrome运行模式
export CHROME_MODE=auto    # 智能混合模式（推荐）
export CHROME_MODE=cold    # 冷启动模式
export CHROME_MODE=warm    # 热启动模式
```

也可以在代码中动态设置：

```go
// 设置全局Chrome模式
utils.SetGlobalChromeMode("auto")
```

## 配置说明

### 环境变量

| 变量名 | 说明 | 示例值 |
|--------|------|--------|
| `WHOISXML_API_KEY` | WhoisXML API密钥 | `your_api_key` |
| `WHOISFREAKS_API_KEY` | WhoisFreaks API密钥 | `your_api_key` |
| `REDIS_ADDR` | Redis服务器地址 | `localhost:6379` |
| `REDIS_PASSWORD` | Redis密码 | `your_password` |
| `PORT` | 服务监听端口 | `3900` |
| `GIN_MODE` | Gin运行模式 | `release` |
| `HEALTH_CHECK_INTERVAL_DAYS` | 健康检查间隔天数 | `1` |
| `CHROME_MODE` | Chrome运行模式 | `auto` (可选: `cold`, `warm`, `auto`) |

## 联系方式

如有任何问题或建议，请通过以下方式联系我们：

- 项目维护者：[AsisYu](https://github.com/AsisYu)