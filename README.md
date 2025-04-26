# 🌐 Whosee-WHOIS 域名信息查询服务

## 📋 项目概述

Whosee-WHOIS是一个高性能的域名WHOIS信息查询服务，提供快速、可靠的域名注册信息查询功能。该服务集成了多个WHOIS数据提供商API，实现了负载均衡、故障转移和智能缓存，确保查询服务的高可用性和响应速度。

前端项目仓库：[https://github.com/AsisYu/whosee-whois](https://github.com/AsisYu/whosee-whois)

## ✨ 主要特性

- 🔄 **多提供商支持**：集成WhoisXML和WhoisFreaks等多个WHOIS数据提供商，提高数据准确性和服务可用性
  > 通过Provider接口抽象不同的WHOIS数据提供商，实现统一调用方式。系统可根据查询需求和提供商状态智能切换数据源，确保获取最准确、最新的域名信息。

- ⚡ **智能缓存系统**：基于Redis的缓存系统，根据域名到期时间动态调整缓存时长
  > 采用多级缓存策略，对查询结果进行智能存储。系统会根据域名到期时间自动计算最优缓存周期，临近过期的域名缓存时间较短，而长期有效的域名则有更长的缓存期，有效减少API调用次数和查询成本。

- ⚖️ **负载均衡**：智能选择最佳WHOIS数据提供商，平衡API调用次数
  > 实现基于权重和可用性的动态负载均衡算法，系统会追踪每个提供商的响应时间、成功率和剩余API配额，自动将查询请求分配给最优的提供商，确保资源利用最大化。

- 🔄 **故障转移**：自动检测API故障并切换到备用提供商
  > 内置健康检查机制，实时监控各提供商API的可用性。当检测到某个提供商服务异常或超时，系统会立即切换到备用提供商，确保服务的连续性和可靠性，用户无感知切换。

- 📊 **统一数据格式**：将不同提供商的数据格式统一，提供一致的API响应
  > 通过数据转换层处理各提供商返回的不同格式数据，提取关键信息并映射到标准化的响应模型。无论底层使用哪个提供商，客户端始终获得结构一致、字段统一的查询结果。

- 🛠️ **完善的错误处理**：详细的错误分类和处理机制
  > 实现分层的错误处理系统，包括网络错误、API限制错误、认证错误等多种类型。每种错误都有明确的错误码和描述信息，便于客户端识别问题并采取相应措施，同时系统会记录详细日志用于问题排查。

- 📸 **增强的截图功能**：智能网站截图与详细错误反馈
  > 提供高级网站截图功能，支持普通网站截图、ITDog测速截图和元素截图。实现了强大的错误处理机制，能够识别并报告DNS解析失败（`net::ERR_NAME_NOT_RESOLVED`）、连接超时、网站拒绝访问等多种错误情况，为用户提供清晰的错误提示和可能的解决方案。采用改进的chromedp实现，通过`chromedp.Evaluate`替代`WaitFunction`，实现更稳定的测速截图功能，并增加了超时保护机制，防止无限等待。前端组件自动在域名变更时清除上一次的截图结果，确保用户体验的一致性。

- 🚦 **API速率限制**：防止API滥用的请求限流机制
  > 采用令牌桶算法实现精确的请求限流控制，可基于IP地址、用户ID或API密钥进行限流。系统支持配置不同用户级别的访问频率，并在请求头中返回限流相关信息，帮助客户端了解当前限制状态。

- 🔒 **安全防护**：包含CORS、请求验证等安全措施
  > 集成多层安全防护机制，包括请求来源验证、参数净化、CSRF防护和API密钥认证。所有API端点都经过安全审计，防止常见的Web攻击，同时支持配置允许的来源域，增强跨域请求的安全性。

## 🏗️ 技术架构

### 💻 核心组件

- 🚀 **Web框架**：基于Gin构建的高性能Web服务
- 📦 **缓存系统**：使用Redis进行数据缓存
- 🔌 **服务管理**：WhoisManager服务管理多个WHOIS提供商
- 🔗 **中间件**：认证、日志、限流、错误处理等中间件
- 🖥️ **前端技术**：基于现代前端框架构建的用户界面，提供直观的域名查询体验

### 📁 目录结构

```
.
├── handlers/       # 请求处理器
├── middleware/     # 中间件组件
├── providers/      # WHOIS数据提供商实现
├── services/       # 核心业务逻辑
├── types/          # 数据类型定义
├── logs/           # 日志文件
├── .env            # 环境变量配置
└── main.go         # 应用入口
```

前端项目位于独立的仓库中：[https://github.com/AsisYu/whosee-whois](https://github.com/AsisYu/whosee-whois)

## 📥 安装指南

### 🔧 前置条件

- 🔹 Go 1.24+
- 🔹 Redis 6.0+
- 🔹 WHOIS API账号（WhoisXML和/或WhoisFreaks）
- 🔹 Chrome/Chromium (用于网站截图功能)

### 📋 安装步骤

1. 克隆仓库

```bash
git clone https://github.com/AsisYu/whosee-whois.git
cd whosee-whois/server
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

### 🖥️ 前端项目安装

1. 克隆前端仓库

```bash
git clone https://github.com/AsisYu/whosee-whois.git
cd whosee-whois
```

2. 安装依赖

```bash
npm install
```

3. 运行开发服务器

```bash
npm run dev
```

## 🚀 部署指南

### 🐳 使用Docker部署

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

### 📦 使用PM2部署（生产环境）

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

### 🌐 Nginx配置（反向代理）

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

### 📋 依赖管理

如需更新依赖，可以使用以下命令：

```bash
# 更新所有依赖到最新版本
go get -u ./...

# 更新特定依赖
go get -u github.com/gin-gonic/gin
go get -u github.com/go-redis/redis/v8
go get -u github.com/chromedp/chromedp

# 整理go.mod文件
go mod tidy
```

### 🔧 截图功能依赖

截图功能依赖于Chrome/Chromium浏览器，在Linux服务器上需要安装以下依赖：

```bash
# Debian/Ubuntu
apt-get update
apt-get install -y wget gnupg
wget -q -O - https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add -
echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google.list
apt-get update
apt-get install -y google-chrome-stable

# CentOS/RHEL
yum install -y wget
wget https://dl.google.com/linux/direct/google-chrome-stable_current_x86_64.rpm
yum install -y google-chrome-stable_current_x86_64.rpm
```

## 🔍 使用示例

### 📡 API接口

#### 查询域名WHOIS信息

```bash
curl -X POST http://localhost:3900/api/whois \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"domain": "example.com"}'
```

#### 响应示例

```json
{
  "available": false,
  "domain": "example.com",
  "registrar": "Example Registrar, LLC",
  "creationDate": "1995-08-14T04:00:00Z",
  "expiryDate": "2023-08-13T04:00:00Z",
  "status": ["clientDeleteProhibited", "clientTransferProhibited"],
  "nameServers": ["ns1.example.com", "ns2.example.com"],
  "updatedDate": "2022-08-14T04:00:00Z",
  "whoisServer": "whois.example-registrar.com",
  "domainAge": 28
}
```

## ⚙️ 配置说明

### 🔐 环境变量

| 变量名 | 说明 | 示例值 |
|--------|------|--------|
| `WHOISXML_API_KEY` | WhoisXML API密钥 | `your_api_key` |
| `WHOISFREAKS_API_KEY` | WhoisFreaks API密钥 | `your_api_key` |
| `REDIS_URL` | Redis连接URL | `redis://:password@localhost:6379/0` |
| `PORT` | 服务监听端口 | `3900` |
| `GIN_MODE` | Gin运行模式 | `release` |

## 🤝 贡献指南

1. Fork本仓库
2. 创建您的特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交您的更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 打开Pull Request

## 📄 许可证

本项目采用MIT许可证 - 详情请参阅 [LICENSE](LICENSE) 文件

## 📞 联系方式

如有任何问题或建议，请通过以下方式联系我们：

- 👨‍💻 项目维护者：[AsisYu](https://github.com/AsisYu)
- 📦 项目仓库：[GitHub](https://github.com/AsisYu/whosee-whois)