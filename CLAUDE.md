# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Whosee.me is a high-performance domain information query and analysis service built with Go 1.24+ and the Gin web framework. It provides WHOIS lookups, DNS queries, website screenshots, and performance testing through a unified API with intelligent caching, load balancing, and circuit breaker protection.

## Core Instruction for CodeX MCP

在任何时刻，你必须思考当前过程可以如何与codex进行协作，如何调用Codex 为你提供的MCP工具作为你客观全面分析的保障。
其中你**务必执行**以下几个步骤：
**1** 在你对用户需求形成初步分析后，将用户需求、初始思路告知codex，并要求其完善需求分析和实施计划。
**2** 在实施具体编码任务前，**必须向codex索要代码实现原型（要求codex仅给出unified diff patch，严禁对代码做任何真实修改）**。在获取代码原型后，你**只能以此为逻辑参考，再次对代码修改进行重写**，形成企业生产级别、可读性极高、可维护性极高的代码后，才能实施具体编程修改任务。
**3** 无论何时，只要完成切实编码行为后，**必须立即使用codex review代码改动和对应需求完成程度**。
**4** codex只能给出参考，你**必须有自己的思考，甚至需要对codex的回答提出置疑**。尽信书则不如无书，你与codex的最终使命都是达成统一、全面、精准的意见，所以你们必须不断争辩已找到通向真理的唯一途径。


## Codex Tool Invocation Specification

 1. 工具概述

  codex MCP 提供了一个工具 `codex`，用于执行 AI 辅助的编码任务。该工具**通过 MCP 协议调用**，无需使用命令行。

  2. 工具参数

  **必选**参数：
  - PROMPT (string): 发送给 codex 的任务指令
  - cd (Path): codex 执行任务的工作目录根路径

  可选参数：
  - sandbox (string): 沙箱策略，可选值：
    - "read-only" (默认): 只读模式，最安全
    - "workspace-write": 允许在工作区写入
    - "danger-full-access": 完全访问权限
  - SESSION_ID (UUID | null): 用于继续之前的会话以与codex进行多轮交互，默认为 None（开启新会话）
  - skip_git_repo_check (boolean): 是否允许在非 Git 仓库中运行，默认 False
  - return_all_messages (boolean): 是否返回所有消息（包括推理、工具调用等），默认 False
  - image (List[Path] | null): 附加一个或多个图片文件到初始提示词，默认为 None
  - model (string | null): 指定使用的模型，默认为 None（使用用户默认配置）
  - yolo (boolean | null): 无需审批运行所有命令（跳过沙箱），默认 False
  - profile (string | null): 从 `~/.codex/config.toml` 加载的配置文件名称，默认为 None（使用用户默认配置）

  返回值：
  {
    "success": true,
    "SESSION_ID": "uuid-string",
    "agent_messages": "agent回复的文本内容",
    "all_messages": []  // 仅当 return_all_messages=True 时包含
  }
  或失败时：
  {
    "success": false,
    "error": "错误信息"
  }

  3. 使用方式

  开启新对话：
  - 不传 SESSION_ID 参数（或传 None）
  - 工具会返回新的 SESSION_ID 用于后续对话

  继续之前的对话：
  - 将之前返回的 SESSION_ID 作为参数传入
  - 同一会话的上下文会被保留

  4. 调用规范

  **必须遵守**：
  - 每次调用 codex 工具时，必须保存返回的 SESSION_ID，以便后续继续对话
  - cd 参数必须指向存在的目录，否则工具会静默失败
  - 严禁codex对代码进行实际修改，使用 sandbox="read-only" 以避免意外，并要求codex仅给出unified diff patch即可

  推荐用法：
  - 如需详细追踪 codex 的推理过程和工具调用，设置 return_all_messages=True
  - 对于精准定位、debug、代码原型快速编写等任务，优先使用 codex 工具

  5. 注意事项

  - 会话管理：始终追踪 SESSION_ID，避免会话混乱
  - 工作目录：确保 cd 参数指向正确且存在的目录
  - 错误处理：检查返回值的 success 字段，处理可能的错误

## Development Commands

### Running the Application

```bash
# Run the server (loads .env automatically)
go run main.go

# Build the binary
go build -o whosee-server main.go

# Run the compiled binary
./whosee-server
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./services/...
go test ./handlers/...

# Run tests with coverage
go test -cover ./...

# Run a specific test
go test -run TestFunctionName ./package/...
```

### Development Tools

```bash
# Install dependencies
go mod download

# Update dependencies
go mod tidy

# View dependencies
go mod graph

# Format code
go fmt ./...

# Run linter (if installed)
golangci-lint run
```

### Docker Operations

```bash
# Build Docker image
docker build -t whosee-server .

# Run with Docker Compose (development)
docker-compose -f docker-compose.dev.yml up

# Run with Docker Compose (production)
docker-compose up -d

# View logs
docker-compose logs -f
```

## Architecture

### Service Container Pattern

All services are centrally managed through `ServiceContainer` in services/container.go. This provides:
- Unified lifecycle management (initialization, shutdown)
- Dependency injection via middleware
- Service discovery

**Key Services:**
- `WhoisManager` - Multi-provider WHOIS query orchestration with failover
- `DNSChecker` - DNS record resolution
- `ScreenshotService` - Unified screenshot service (refactored architecture)
- `ChromeManager` - Chrome instance pool with circuit breaker protection
- `WorkerPool` - CPU-based concurrent task execution
- `HealthChecker` - Periodic health monitoring for all services
- `RateLimiter` - Redis-backed distributed rate limiting

### Provider Pattern

External APIs implement the `WhoisProvider` interface (types/whois.go):

```go
type WhoisProvider interface {
    Query(domain string) (*WhoisResponse, error, bool)
    Name() string
}
```

Providers are registered with `WhoisManager`, which handles:
- Load balancing across providers
- Automatic failover on errors
- API call tracking
- Circuit breaker integration

**Available Providers:**
- WhoisFreaks (providers/whoisfreaks.go)
- WhoisXML (providers/whoisxml.go)
- IANA RDAP (providers/iana_rdap.go)
- IANA WHOIS (providers/iana_whois.go)

### Screenshot Service Architecture (Refactored)

The screenshot system was recently refactored with a unified architecture:

**Core Components:**
- `ScreenshotService` (services/screenshot_service.go) - Unified service supporting all screenshot types (basic, element, ITDog variants)
- `ChromeManager` (services/chrome_manager.go) - Manages Chrome instances with intelligent concurrency control (max 3 slots), circuit breaker protection, and automatic recovery
- Handlers: `screenshot_new.go` (new unified API) maintains backward compatibility with `screenshot.go` (legacy)

**Screenshot Types:**
- `basic` - Full page screenshot
- `element` - Specific element via CSS/XPath selector
- `itdog_map` - ITDog performance map
- `itdog_table` - ITDog results table
- `itdog_ip` - ITDog IP statistics
- `itdog_resolve` - ITDog comprehensive speed test

**Chrome Management Modes:**
- `cold` - Start fresh per request (lowest memory, slowest)
- `warm` - Keep-alive with health monitoring (fastest, higher memory)
- `auto` - Intelligent hybrid mode (recommended, balances speed/resources)

Set via environment variable: `CHROME_MODE=auto|cold|warm`

### Security & Authentication System

Multi-layer security implemented through middleware chain:

1. **JWT Token Authentication** (middleware/auth.go)
   - Short-lived tokens (30 seconds)
   - Nonce-based replay protection (Redis-backed)
   - IP binding for tokens
   - Obtain token: `POST /api/auth/token`

2. **API Key Authentication**
   - Long-term access via `X-API-KEY` header or `apikey` query param
   - Configured with `API_KEY` environment variable

3. **IP Whitelist** (middleware/ip_whitelist.go)
   - Redis-cached validation
   - Two modes:
     - Strict (`IP_WHITELIST_STRICT_MODE=true`): IP whitelist AND API key required
     - Permissive (`IP_WHITELIST_STRICT_MODE=false`): IP whitelist OR API key (default)
   - Development bypass: `API_DEV_MODE=true`

4. **Rate Limiting** (middleware/ratelimit.go)
   - Redis-based distributed limiting
   - Sliding window algorithm
   - Configurable per endpoint

5. **Security Headers** (middleware/security.go)
   - CSP, HSTS, X-Frame-Options, etc.

**Authentication Flow:**
```bash
# Get token
TOKEN=$(curl -X POST http://localhost:3900/api/auth/token | jq -r '.token')

# Use token + API key
curl -H "Authorization: Bearer $TOKEN" \
     -H "X-API-KEY: your_api_key" \
     http://localhost:3900/api/v1/whois/example.com
```

See docs/BACKEND_AUTHENTICATION_FLOW.md for detailed flow diagrams.

### Caching Strategy

Redis-based caching with intelligent TTL:

**Cache Key Patterns:**
- `whois:{domain}` - WHOIS data (TTL based on domain expiry)
- `dns:{domain}` - DNS records (30 minutes)
- `screenshot:{domain}:{type}` - Screenshots (24 hours default, configurable)
- `health:*` - Health check results (5 minutes)

**Implementation:**
- Cache keys generated via utils/cache_keys.go
- Automatic cache invalidation
- Cache warming for health checks

### Circuit Breaker Pattern

Protects against cascading failures:

- Located in services/circuit_breaker.go and services/service_breakers.go
- Used by ChromeManager, WHOIS providers, external APIs
- States: Closed (normal) → Open (failing) → Half-Open (testing recovery)
- Configurable failure thresholds and timeout durations

### Health Monitoring

Unified health check system (services/health_checker.go):

- Endpoint: `GET /api/health?detailed=true`
- Monitors: WHOIS providers, DNS, Screenshots, ITDog, Chrome, Redis
- Separate health logs: `logs/health_YYYY-MM-DD.log` (when `HEALTH_LOG_SEPARATE=true`)
- Periodic background checks with forced refresh on startup

## Code Organization

```
handlers/          # HTTP request handlers (thin layer)
├── screenshot_new.go     # Unified screenshot API (refactored)
├── screenshot.go         # Legacy screenshot handlers (backward compatible)
├── whois.go             # WHOIS query handlers
├── dns.go               # DNS query handlers
└── health.go            # Health check endpoint

services/          # Business logic layer
├── container.go          # Service container and lifecycle management
├── screenshot_service.go # Unified screenshot service
├── chrome_manager.go     # Chrome instance pool with circuit breaker
├── whois_manager.go      # Multi-provider WHOIS orchestration
├── circuit_breaker.go    # Circuit breaker implementation
└── worker_pool.go        # Concurrent task execution

middleware/        # Cross-cutting concerns
├── auth.go              # JWT + API key authentication
├── ip_whitelist.go      # IP access control
├── ratelimit.go         # Distributed rate limiting
├── security.go          # Security headers
├── logging.go           # Structured logging
└── service.go           # Service container injection

providers/         # External API integrations
├── whoisfreaks.go       # WhoisFreaks provider
├── whoisxml.go          # WhoisXML provider
├── iana_rdap.go         # IANA RDAP provider
└── iana_whois.go        # IANA WHOIS provider

routes/            # API route definitions
├── routes.go            # Main route registration
└── screenshot_routes.go # Screenshot-specific routes

utils/             # Utility functions
├── chrome.go            # Chrome utility (smart mode management)
├── chrome_downloader.go # Automatic Chrome download/platform detection
├── domain.go            # Domain validation and security
├── api.go               # Standardized API responses
├── cache_keys.go        # Cache key generation
└── health_logger.go     # Health check logging

types/             # Data structures
└── whois.go             # WHOIS types and interfaces
```

## Important Conventions

### Error Handling

Use standardized error responses (utils/api.go):

```go
// Standard error response
utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_DOMAIN", "Invalid domain format")

// Success response
utils.SuccessResponse(c, data)
```

**Standard Error Codes:**
- `MISSING_PARAMETER` - Required parameter missing
- `INVALID_DOMAIN` - Domain format invalid
- `INVALID_URL` - URL security check failed
- `RATE_LIMITED` - Rate limit exceeded
- `SERVICE_BUSY` - Circuit breaker open
- `TIMEOUT` - Request timeout
- `QUERY_ERROR` - Generic query error
- `SCREENSHOT_ERROR` - Screenshot operation failed
- `CHROME_ERROR` - Chrome instance error

### Logging Standards

- Use structured logging with context
- Health checks can log separately (configure `HEALTH_LOG_SEPARATE=true`)
- Include request IDs and relevant metadata
- Log levels: INFO (normal ops), WARN (recoverable issues), ERROR (failures)

### Naming Conventions

- Files: snake_case (e.g., `screenshot_service.go`)
- Exported types: PascalCase (e.g., `ScreenshotService`)
- Unexported functions: camelCase (e.g., `generateCacheKey`)
- Constants: UPPER_CASE (e.g., `DEFAULT_TIMEOUT`)

## Environment Configuration

### Required Variables

```bash
# API Keys
WHOISXML_API_KEY=your_key
WHOISFREAKS_API_KEY=your_key
API_KEY=your_key           # For API authentication
JWT_SECRET=your_secret     # For JWT signing

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=            # Optional

# Server
PORT=3900
GIN_MODE=release          # or "debug"
```

### Optional Variables

```bash
# Chrome Configuration
CHROME_MODE=auto          # auto (default), cold, warm

# Security
DISABLE_API_SECURITY=false
IP_WHITELIST_STRICT_MODE=false
API_DEV_MODE=false
TRUSTED_IPS=              # Comma-separated IPs

# Health Checks
HEALTH_LOG_SEPARATE=false
HEALTH_LOG_SILENT=false

# CORS
CORS_ORIGINS=http://localhost:3000,https://whosee.me
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE
```

## Chrome Management

Chrome is automatically downloaded on first run if not found. The system detects platform and uses China mirror sources when needed.

**Chrome Initialization:**
- Runs asynchronously in background (non-blocking startup)
- Auto-download with platform detection (utils/chrome_downloader.go)
- Health monitoring starts after initialization
- Graceful degradation if Chrome unavailable

**When Chrome Fails:**
- Screenshot endpoints return appropriate errors
- Other services (WHOIS, DNS) remain fully functional
- Health endpoint reports Chrome status
- Logs contain detailed error information

## Adding New Features

### New API Endpoint

1. Define route in routes/routes.go or routes/api.go
2. Create handler in handlers/ (thin layer, delegates to service)
3. Implement business logic in services/
4. Add request/response types in types/
5. Update error codes in utils/api.go if needed
6. Consider caching strategy (utils/cache_keys.go)
7. Add circuit breaker if calling external service

### New WHOIS Provider

1. Create provider file in providers/
2. Implement `WhoisProvider` interface
3. Register in main.go with `serviceContainer.WhoisManager.AddProvider()`
4. Add environment variables for API keys
5. Test failover behavior

### New Middleware

1. Create file in middleware/
2. Follow config struct pattern (e.g., `MiddlewareConfig`)
3. Return `gin.HandlerFunc`
4. Consider middleware chain order (see middleware-security.mdc)
5. Document in middleware/README.md

## Testing Strategy

### Unit Tests
- Mock external dependencies (Redis, providers, Chrome)
- Focus on business logic in services/
- Test error paths and edge cases

### Integration Tests
- Use test Redis instance
- Test middleware chains
- Verify service interactions
- Test circuit breaker behavior

### Manual Testing
- Health endpoint: `curl http://localhost:3900/api/health?detailed=true`
- WHOIS: `curl http://localhost:3900/api/v1/whois/google.com`
- Screenshot: `curl -X POST http://localhost:3900/api/v1/screenshot/ -d '{"type":"basic","domain":"example.com","format":"file"}'`

## Common Development Tasks

### Debugging Authentication Issues
1. Check `API_DEV_MODE=true` for development (bypasses IP whitelist)
2. Verify JWT_SECRET and API_KEY are set
3. Check IP whitelist mode: `IP_WHITELIST_STRICT_MODE`
4. Review logs in logs/server_YYYY-MM-DD.log
5. See docs/AUTHENTICATION_EXAMPLES.md for examples

### Debugging Screenshot Issues
1. Check Chrome status: `GET /api/v1/screenshot/chrome/status`
2. Restart Chrome: `POST /api/v1/screenshot/chrome/restart`
3. Verify CHROME_MODE setting
4. Check logs for Chrome initialization
5. Ensure chrome_runtime/ directory has proper permissions

### Performance Optimization
- Monitor cache hit rates via logs
- Adjust Redis connection pool settings (main.go)
- Tune worker pool size based on CPU cores
- Review circuit breaker thresholds
- Check rate limit configurations

### Deployment
- Use Docker: `docker-compose.yml` for production
- Set `GIN_MODE=release`
- Configure appropriate CORS origins
- Enable security features (disable dev mode)
- Set up proper Redis persistence
- Monitor health endpoint

## Key Files Reference

- main.go:344 - Application entry point, initialization sequence
- services/container.go:89 - Service lifecycle management
- services/screenshot_service.go - Unified screenshot implementation
- services/chrome_manager.go - Chrome pool and circuit breaker
- middleware/auth.go - Authentication middleware
- middleware/ip_whitelist.go - Access control
- routes/routes.go - API route definitions
- utils/api.go - Standard response formats
- docs/BACKEND_AUTHENTICATION_FLOW.md - Authentication flow diagrams
- docs/SCREENSHOT_REFACTOR.md - Screenshot service architecture
