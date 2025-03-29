# 🌐 Whosee-WHOIS 域名信息查询服务

## 📋 项目概述

Whosee-WHOIS是一个高性能的域名WHOIS信息查询服务，提供快速、可靠的域名注册信息查询功能。该服务集成了多个WHOIS数据提供商API，实现了负载均衡、故障转移和智能缓存，确保查询服务的高可用性和响应速度。

前端项目仓库：[https://github.com/AsisYu/whosee-whois](https://github.com/AsisYu/whosee-whois)

## ✨ 主要特性

- 🔄 **多提供商支持**
- ⚡ **智能缓存系统**
- ⚖️ **负载均衡**
- 🔄 **故障转移**
- 📊 **统一数据格式**
- 🛠️ **完善的错误处理**
- 🚦 **API速率限制**
- 🔒 **安全防护**

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

- 🔹 Go 1.16+
- 🔹 Redis 6.0+
- 🔹 WHOIS API账号（WhoisXML和/或WhoisFreaks）

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
```

2. 安装依赖

```bash
cd whosee-whois
npm install
```

3. 启动开发服务器

```bash
npm run dev
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