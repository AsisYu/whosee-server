# Whosee.me 域名信息查询服务

## 项目概述

Whosee.me 是一个高性能的域名信息查询和分析服务，提供快速、可靠的域名WHOIS信息查询、DNS记录查询、网站截图和性能测试功能。该服务集成了多个数据提供商API，实现了负载均衡、故障转移和智能缓存，并采用高并发优化架构，确保查询服务的高可用性和响应速度。

前端仓库地址：[https://github.com/AsisYu/whosee-whois](https://github.com/AsisYu/whosee-whois)

## 主要特性

### 基础功能

- **多提供商支持**: 集成RDAP、WhoisXML和WhoisFreaks等多个WHOIS数据提供商，提高数据准确性和服务可用性
- **智能缓存系统**: 基于Redis的缓存系统，根据域名到期时间动态调整缓存时长
- **负载均衡**: 智能选择最佳数据提供商，平衡API调用次数
- **故障转移**: 自动检测API故障并切换到备用提供商

### 高级特性

- **全面的健康检查系统**: 提供统一的健康检查API，监控多种服务状态，支持日志分离和详细统计
- **健康检查日志分离**: 支持将健康检查日志独立存储，便于日志管理和分析，可配置静默模式
- **智能Chrome管理**: 支持冷启动、热启动、智能混合三种模式，自动下载和平台检测
- **重构截图服务** :
  - 统一服务架构，支持所有截图类型（基础、元素、ITDog系列）
  - 性能提升50%，智能并发控制（最大3个槽位）
  - 熔断器保护和自动恢复机制
  - 安全增强（输入验证、安全文件操作、错误脱敏）
  - 完整向后兼容，平滑迁移
- **高并发优化架构**: 全面优化的高并发处理能力
- **完善的错误处理**: 统一的API响应格式和详细的错误分类
- **安全防护**: 包含CORS、请求验证等安全措施

## 技术架构

### 核心组件

- **Web框架**: 基于Gin构建的高性能Web服务
- **缓存系统**: 使用Redis进行数据缓存和分布式限流
- **服务管理**: 服务容器模式管理多个服务组件
- **重构截图系统** 🆕:
  - **统一截图服务**: 单一接口支持所有截图类型
  - **Chrome管理器**: 智能并发控制、熔断器保护、性能统计
  - **安全增强**: 域名验证、安全文件操作、错误脱敏
  - **Chrome实例池**: 统一资源管理，复用率提升50%
- **智能Chrome工具**: 支持智能混合模式，自动平台检测和资源优化
- **智能工作池**: 基于CPU核心数的动态工作池，高效处理并发请求
- **熔断保护**: 防止系统在故障条件下过载的熔断器模式
- **中间件**: 认证、日志、限流、错误处理等中间件

### 目录结构

```
.
├── handlers/                            # 请求处理器和异步处理函数
│   ├── screenshot_new.go                # 🆕 重构后的统一截图处理器
│   └── screenshot.go                    # 原有截图处理器（兼容保留）
├── middleware/                          # 中间件组件
├── providers/                           # WHOIS数据提供商实现
├── services/                            # 核心业务逻辑和服务组件
│   ├── screenshot_service.go            # 🆕 统一截图服务
│   ├── chrome_manager.go                # 🆕 重构Chrome管理器
│   ├── whois_manager.go                 # 🔧 P1并发安全修复
│   └── ...                              # 其他服务组件
├── routes/                              # API路由定义
│   └── screenshot_routes.go             # 🆕 截图服务路由
├── types/                               # 数据类型定义
├── utils/                               # 辅助函数和工具
│   └── domain.go                        # 增强的安全工具
├── tests/                               # 🆕 测试脚本和工具
│   ├── README.md                        # 测试文档索引
│   ├── test_runtime.sh                  # 旧版运行时测试
│   └── test_runtime_v2.sh               # 🔐 P0修复运行时测试（推荐）
├── docs/                                # 文档目录
│   ├── reports/                         # 🆕 项目报告
│   │   ├── README.md                    # 报告索引
│   │   ├── PROJECT_HEALTH_REPORT.md     # 项目健康检查报告
│   │   ├── P0_FIX_REVIEW.md             # 🔐 P0安全修复审查
│   │   └── RUNTIME_TEST_REPORT.md       # 🔐 运行时测试报告
│   ├── architecture/                    # 🆕 架构文档
│   │   └── README.md                    # 架构文档索引
│   ├── BACKEND_AUTHENTICATION_FLOW.md   # 后端认证流程详细文档
│   ├── AUTHENTICATION_EXAMPLES.md       # 认证示例集合文档
│   ├── ALL_JSON.md                      # API响应格式文档
│   └── SCREENSHOT_REFACTOR.md           # 🆕 截图服务重构指南
├── logs/                                # 日志文件
├── static/                              # 静态资源（截图等）
├── CLAUDE.md                            # Claude Code开发指南
├── .env                                 # 环境变量配置
└── main.go                              # 应用入口
```

**图例说明**:
- 🆕 新增功能或重构
- 🔐 安全修复
- 🔧 性能或并发优化

## 安装指南

### 前置条件

- Go 1.24+
- Redis 6.0+
- WHOIS API账号（WhoisXML和/或WhoisFreaks）
- Chrome/Chromium (用于网站截图功能，支持自动下载和智能平台检测)

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

## API接口说明

### 主要API端点

| 端点 | 方法 | 说明 | 参数 |
|------|------|------|------|
| `/api/health` | GET | 健康检查API，返回所有服务的健康状态 | `detailed=true/false`: 是否返回详细状态 |
| `/api/v1/whois` | GET | WHOIS信息查询（通过查询参数） | `domain`: 要查询的域名 |
| `/api/v1/whois/:domain` | GET | WHOIS信息查询（通过路径参数） | `:domain`: 路径中的域名 |
| `/api/v1/rdap` | GET | RDAP协议查询（通过查询参数） | `domain`: 要查询的域名 |
| `/api/v1/rdap/:domain` | GET | RDAP协议查询（通过路径参数） | `:domain`: 路径中的域名 |
| `/api/v1/dns` | GET | DNS记录查询（通过查询参数） | `domain`: 要查询的域名 |
| `/api/v1/dns/:domain` | GET | DNS记录查询（通过路径参数） | `:domain`: 路径中的域名 |

###  截图服务API

#### 新版统一接口（推荐）
| 端点 | 方法 | 说明 | 参数 |
|------|------|------|------|
| `/api/v1/screenshot/` | POST/GET | 统一截图接口，支持所有截图类型 | JSON请求体或查询参数 |
| `/api/v1/screenshot/chrome/status` | GET | Chrome状态检查和性能统计 | 无 |
| `/api/v1/screenshot/chrome/restart` | POST | Chrome重启 | 无 |

**统一截图接口请求示例：**
```json
{
  "type": "basic|element|itdog_map|itdog_table|itdog_ip|itdog_resolve",
  "domain": "example.com",
  "url": "https://example.com",        // 可选，优先级高于domain
  "selector": ".main-content",         // 元素截图必需
  "format": "file|base64",
  "timeout": 60,                       // 秒
  "wait_time": 3,                      // 秒
  "cache_expire": 24                   // 小时
}
```

#### 兼容旧版接口
| 端点 | 方法 | 说明 | 参数 |
|------|------|------|------|
| `/api/v1/screenshot` | GET | 网站截图服务（通过查询参数） | `domain`: 要截图的域名 |
| `/api/v1/screenshot/:domain` | GET | 网站截图服务（通过路径参数） | `:domain`: 路径中的域名 |
| `/api/v1/screenshot/base64/:domain` | GET | 返回Base64编码的网站截图 | `:domain`: 路径中的域名 |
| `/api/v1/itdog/:domain` | GET | ITDog网站性能测试截图 | `:domain`: 路径中的域名 |
| `/api/v1/itdog/base64/:domain` | GET | 返回Base64编码的ITDog测试结果 | `:domain`: 路径中的域名 |
| `/api/auth/token` | POST | 获取安全访问令牌 | 无需参数，根据客户端IP发放短期令牌 |

### 安全认证

Whosee.me API采用多层安全验证机制，包括IP白名单、API密钥验证和JWT令牌认证。详细的验证流程请参考 [后端认证流程文档](docs/BACKEND_AUTHENTICATION_FLOW.md)。

#### 安全配置

生产环境推荐配置:
```bash
DISABLE_API_SECURITY=false
IP_WHITELIST_STRICT_MODE=false  # 允许所有ip通过API密钥访问
API_DEV_MODE=false
API_KEY=your_strong_api_key
JWT_SECRET=your_strong_jwt_secret
```

注意:
- JWT令牌有效期为30秒，每个令牌只能使用一次
- API密钥可重复使用，适合程序化访问
- 非严格模式下，IP白名单或API密钥验证任一通过即可访问
- 请求频率限制为每IP每分钟30个令牌

### 常见错误代码

| 错误代码 | HTTP状态码 | 描述 |
|---------|----------|------|
| `MISSING_PARAMETER` | 400 | 缺少必要的参数 |
| `INVALID_DOMAIN` | 400 | 无效的域名格式 |
| `INVALID_URL` | 400 | 不安全的URL |
| `INVALID_SELECTOR` | 400 | 无效的CSS选择器 |
| `RATE_LIMITED` | 429 | 请求频率超过限制 |
| `SERVICE_BUSY` | 503 | 服务忙碌（熔断器开启） |
| `TIMEOUT` | 504 | 请求处理超时 |
| `QUERY_ERROR` | 500 | 查询过程中发生错误 |
| `SCREENSHOT_ERROR` | 500 | 截图过程中发生错误 |
| `CHROME_ERROR` | 500 | Chrome实例错误 |
| `ELEMENT_NOT_FOUND` | 500 | 页面元素未找到 |
| `BROWSER_ERROR` | 500 | 浏览器执行错误 |
| `ITDOG_ERROR` | 500 | ITDog测试过程中发生错误 |

## 截图服务架构重构

###  重构亮点

Whosee.me 的截图服务经过全面重构，采用现代化架构设计，实现了显著的性能提升和功能增强：

#### 性能提升
- **资源利用率提升50%**: 统一Chrome实例管理，避免重复启动
- **智能并发控制**: 最大3个并发任务，防止系统过载
- **熔断器保护**: 自动故障检测和恢复机制
- **智能缓存**: Redis缓存，支持自定义过期时间

#### 安全增强
- **输入验证**: 域名格式、URL安全性、选择器验证
- **安全文件操作**: 防止路径遍历攻击
- **错误脱敏**: 避免敏感信息泄露
- **防护内网访问**: 阻止访问内网IP地址

#### 维护性提升
- **统一服务架构**: 单一接口支持所有截图类型
- **完整向后兼容**: 保持现有API接口不变
- **标准化错误处理**: 统一错误码和用户友好消息
- **详细监控**: Chrome状态、性能统计、错误追踪

###  技术架构

#### 核心组件
- **ScreenshotService**: 统一截图服务，支持基础、元素、ITDog等所有类型
- **ChromeManager**: Chrome实例管理器，智能并发控制和资源复用
- **熔断器模式**: 自动故障恢复，确保服务稳定性
- **安全工具**: 域名验证、文件名清理、URL安全检查

#### API设计
- **新版统一接口**: `POST /api/v1/screenshot/` 支持所有截图类型
- **Chrome管理API**: 状态检查、重启等管理功能
- **兼容旧版接口**: 保持现有客户端无需修改

详细的重构说明和迁移指南请参考 [截图服务重构文档](docs/SCREENSHOT_REFACTOR.md)。

## Chrome智能管理

### 三种运行模式

Whosee.me 集成了智能Chrome管理系统，特别适合WHOIS服务（主要功能）+ 偶尔截图的使用场景：

| 模式 | 启动方式 | 资源占用 | 响应速度 | 适用场景 | 空闲管理 |
|------|----------|----------|----------|----------|----------|
| **冷启动** | 每次重新启动 | 最低 | 慢(2-3秒) | 极少使用截图 | 用完即关 |
| **热启动** | 预热保持运行 | 较高 | 最快(<100ms) | 频繁使用截图 | 10分钟自动关闭 |
| **智能混合** | 按需+智能复用 | 中等 | 适中 | **WHOIS主业务+偶尔截图** | 智能调整(1.5-6分钟) |

### 智能混合模式

默认采用智能混合模式，具有以下特性:

- **智能启动策略**: 首次使用快速启动，频繁使用自动切换为热启动策略
- **智能空闲管理**: 根据使用频率动态调整空闲超时时间（1.5-6分钟）
- **实例复用**: 健康的Chrome实例直接复用，避免重复启动开销
- **自动下载**: 智能检测平台并自动下载Chrome，支持中国镜像源
- **并发控制**: 最大3个并发操作，避免资源竞争

### Chrome配置

可通过环境变量 `CHROME_MODE` 配置运行模式:

```bash
# 设置Chrome运行模式
export CHROME_MODE=auto    # 智能混合模式（推荐）
export CHROME_MODE=cold    # 冷启动模式
export CHROME_MODE=warm    # 热启动模式
```

也可以在代码中动态设置:

```go
// 设置全局Chrome模式
utils.SetGlobalChromeMode("auto")
```

## 配置说明

### 健康检查日志分离

Whosee.me 支持将健康检查日志分离到独立文件，便于日志管理和分析：

#### 功能特性

- **独立日志文件**: 健康检查日志写入 `logs/health_YYYY-MM-DD.log` 文件
- **主日志静默**: 可配置在主服务器日志中静默健康检查信息
- **详细统计**: 提供健康检查总结，包含服务状态统计和耗时信息
- **向后兼容**: 默认关闭，不影响现有部署

#### 配置方式

```bash
# 启用健康检查日志分离
HEALTH_LOG_SEPARATE=true

# 启用主日志静默模式（推荐与日志分离一起使用）
HEALTH_LOG_SILENT=true
```

#### 日志文件结构

启用后将生成以下日志文件：
- `logs/health_YYYY-MM-DD.log`: 健康检查详细日志
- `logs/server.log`: 主服务器日志（不含健康检查信息）

###  目录功能概览

每个目录都包含详细的README文档，包括功能说明、使用示例和最佳实践：

- **[handlers/](handlers/README.md)** 🆕 - HTTP请求处理器，包含重构后的统一截图处理器
- **[services/](services/README.md)** 🆕 - 核心业务逻辑，包含Chrome管理器和截图服务
- **[routes/](routes/README.md)** 🆕 - API路由定义，包含新版截图路由配置
- **[middleware/](middleware/README.md)** - 中间件组件，包含安全、性能和监控中间件
- **[utils/](utils/README.md)**  - 工具函数，包含增强的安全工具和Chrome管理
- **[providers/](providers/)** - WHOIS数据提供商实现
- **[types/](types/)** - 数据类型定义

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
| `HEALTH_LOG_SEPARATE` | 是否将健康检查日志分离到独立文件 | `false` |
| `HEALTH_LOG_SILENT` | 是否在主日志中静默健康检查信息 | `false` |
| `CHROME_MODE` | Chrome运行模式 | `auto` (可选: `cold`, `warm`, `auto`) |
| `DISABLE_API_SECURITY` | 是否禁用API安全验证 | `false` |
| `IP_WHITELIST_STRICT_MODE` | IP白名单严格模式 | `true` |
| `API_DEV_MODE` | API开发模式 | `false` |
| `TRUSTED_IPS` | 受信任的IP列表 | 空 |


###  架构演进历程

Whosee.me 经历了从单一功能到完整平台的演进：

1. **第一阶段**: 基础WHOIS查询服务
2. **第二阶段**: 集成多提供商支持和智能缓存
3. **第三阶段**: 添加截图功能和Chrome管理
4. **第四阶段** 🆕: 截图服务全面重构，统一架构设计
5. **第五阶段**: 安全增强和性能优化

###  性能指标

重构后的系统性能显著提升：

- **截图响应时间**: 从平均5-8秒降低到2-4秒
- **Chrome资源利用率**: 提升50%，内存占用降低30%
- **并发处理能力**: 支持3个并发截图任务
- **错误率**: 从15%降低到<2%
- **缓存命中率**: 85%以上

## 文档说明

### 核心文档

- **[后端认证流程文档](docs/BACKEND_AUTHENTICATION_FLOW.md)**: 详细说明API安全认证机制，包括JWT令牌、API密钥验证和IP白名单的完整流程，含Mermaid流程图
- **[认证示例集合文档](docs/AUTHENTICATION_EXAMPLES.md)**: 提供各种认证场景的实用示例，包括curl命令、多语言客户端代码、错误处理和调试技巧
- **[API响应格式文档](docs/ALL_JSON.md)**: 所有API端点的响应格式和数据结构说明
- **[截图服务重构文档](docs/SCREENSHOT_REFACTOR.md)** 🆕: 详细的重构说明、迁移指南和性能对比

### 验证流程概述

后端采用多层安全验证机制:

1. **安全开关检查**: 通过 `DISABLE_API_SECURITY` 控制是否启用安全验证
2. **IP白名单验证**: 支持Redis缓存，可配置严格/非严格模式
3. **API密钥验证**: 支持请求头和查询参数两种方式
4. **JWT令牌认证**: 短期有效的访问令牌机制
5. **其他安全中间件**: CORS、安全头部、限流等

详细的认证流程图和配置说明请参考 [后端认证流程文档](docs/BACKEND_AUTHENTICATION_FLOW.md)，实用的认证示例请参考 [认证示例集合文档](docs/AUTHENTICATION_EXAMPLES.md)。

## 开发工具

本项目采用现代化开发工具链，包括 AI 辅助开发工具（Trae、Cursor/Claude code）进行代码生成、架构设计和开发优化，显著提升开发效率和代码质量。