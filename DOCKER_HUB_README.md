# Whosee Server - Domain Information Query Service

A high-performance domain information query and analysis service providing WHOIS lookups, DNS queries, website screenshots, and performance testing through a unified API.

## Features

### Core Capabilities

- **Multi-Provider WHOIS**: Integrated with RDAP, WhoisXML, and WhoisFreaks APIs
- **Intelligent Caching**: Redis-based caching with dynamic TTL based on domain expiry
- **Load Balancing**: Smart provider selection and automatic failover
- **DNS Queries**: Comprehensive DNS record resolution
- **Website Screenshots**: Full-page, element, and ITDog performance screenshots
- **Performance Testing**: ITDog integration for multi-region speed tests

### Advanced Features

- **Chrome Instance Pool**: Intelligent concurrency control (max 3 slots)
- **Circuit Breaker**: Automatic failure detection and recovery
- **Health Monitoring**: Unified health check system for all services
- **Security**: JWT authentication, API key validation, rate limiting, IP whitelist
- **High Concurrency**: Optimized worker pool based on CPU cores

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Create docker-compose.yml
version: '3.8'

services:
  whosee-server:
    image: hansomeyu/whosee-server:latest
    container_name: whosee-server
    ports:
      - "3900:3900"
    environment:
      - JWT_SECRET=your_jwt_secret
      - API_KEY=your_api_key
      - WHOISFREAKS_API_KEY=your_whoisfreaks_key
      - WHOISXML_API_KEY=your_whoisxml_key
      - REDIS_ADDR=redis:6379
      - CHROME_MODE=auto
      - GIN_MODE=release
    depends_on:
      - redis
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    container_name: whosee-redis
    restart: unless-stopped
    volumes:
      - redis-data:/data

volumes:
  redis-data:
```

```bash
# Start services
docker-compose up -d
```

### Using Docker Run

```bash
# Pull the image
docker pull hansomeyu/whosee-server:latest

# Run container
docker run -d -p 3900:3900 --name whosee-server \
  -e JWT_SECRET=your_jwt_secret \
  -e API_KEY=your_api_key \
  -e WHOISFREAKS_API_KEY=your_key \
  -e WHOISXML_API_KEY=your_key \
  -e REDIS_ADDR=redis:6379 \
  --restart unless-stopped \
  hansomeyu/whosee-server:latest
```

## Required Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `JWT_SECRET` | Secret key for JWT token signing | Yes |
| `API_KEY` | API key for authentication | Yes |
| `REDIS_ADDR` | Redis server address | Yes |
| `WHOISFREAKS_API_KEY` | WhoisFreaks API key | Optional |
| `WHOISXML_API_KEY` | WhoisXML API key | Optional |

## Optional Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3900` | Server port |
| `GIN_MODE` | `release` | Gin mode (release/debug) |
| `CHROME_MODE` | `auto` | Chrome mode (auto/cold/warm) |
| `API_DEV_MODE` | `false` | Development mode (bypasses auth) |
| `DISABLE_API_SECURITY` | `false` | Disable all security checks |
| `IP_WHITELIST_STRICT_MODE` | `false` | Require both IP and API key |
| `HEALTH_LOG_SEPARATE` | `false` | Separate health check logs |

## API Endpoints

### Authentication

```bash
# Get JWT token
POST /api/auth/token
```

### WHOIS Query

```bash
# Query domain WHOIS information
GET /api/v1/whois/:domain
Authorization: Bearer <token>
X-API-KEY: <api_key>
```

### DNS Query

```bash
# Query DNS records
GET /api/v1/dns/:domain
Authorization: Bearer <token>
X-API-KEY: <api_key>
```

### Screenshot Service

```bash
# Unified screenshot API
POST /api/v1/screenshot/
Content-Type: application/json
Authorization: Bearer <token>
X-API-KEY: <api_key>

{
  "type": "basic|element|itdog_map|itdog_table|itdog_ip|itdog_resolve",
  "domain": "example.com",
  "format": "file|base64"
}
```

### Health Check

```bash
# Basic health check
GET /api/health

# Detailed health check
GET /api/health?detailed=true
```

## Chrome Management

The service includes intelligent Chrome instance management:

**Modes:**
- `cold`: Start fresh Chrome for each request (lowest memory, slowest)
- `warm`: Keep Chrome running (fastest, higher memory)
- `auto`: Intelligent hybrid mode (recommended, balances speed and resources)

Set via environment variable: `CHROME_MODE=auto`

**Chrome Status:**
```bash
GET /api/v1/screenshot/chrome/status
```

**Restart Chrome:**
```bash
POST /api/v1/screenshot/chrome/restart
```

## Image Information

**Supported Platforms:**
- linux/amd64 (x86_64)
- linux/arm64 (ARM64/Apple Silicon)

**Image Size:**
- Compressed: ~250-300MB
- Uncompressed: ~800MB-1GB (includes Chromium browser)

**Included Components:**
- Go 1.24.11 runtime
- Chromium browser
- Chinese font support (wqy-zenhei)
- Health check utilities

## Security Features

### Multi-Layer Authentication

1. **JWT Token**: Short-lived tokens (30 seconds) with nonce-based replay protection
2. **API Key**: Long-term access via X-API-KEY header
3. **IP Whitelist**: Redis-cached IP validation with two modes
4. **Rate Limiting**: Distributed rate limiting with sliding window

### Security Headers

- CSP (Content Security Policy)
- HSTS (HTTP Strict Transport Security)
- X-Frame-Options
- X-Content-Type-Options
- Referrer-Policy
- Permissions-Policy

## Health Monitoring

The service provides comprehensive health checks:

```json
{
  "status": "healthy|degraded|unhealthy",
  "services": {
    "whois": "healthy",
    "dns": "healthy",
    "screenshot": "healthy",
    "chrome": "healthy",
    "redis": "healthy"
  },
  "timestamp": "2025-12-04T10:00:00Z"
}
```

## Performance

- **Concurrent Requests**: CPU-based worker pool
- **Screenshot Performance**: 50% faster with instance pooling
- **Cache Hit Rate**: 90%+ for repeated queries
- **Response Time**: <100ms for cached queries

## Troubleshooting

### Container fails to start

1. Check Redis connectivity
2. Verify all required environment variables are set
3. Check logs: `docker logs whosee-server`

### Chrome screenshots not working

1. Check Chrome status: `GET /api/v1/screenshot/chrome/status`
2. Restart Chrome: `POST /api/v1/screenshot/chrome/restart`
3. Verify CHROME_MODE setting
4. Check container logs for Chrome errors

### Authentication errors

1. Ensure JWT_SECRET and API_KEY are set
2. Check token expiry (30 seconds)
3. Verify API key matches configuration
4. For development, set API_DEV_MODE=true

## Development

**Run from source:**

```bash
git clone https://github.com/AsisYu/whosee-server.git
cd whosee-server
cp .env.example .env
# Edit .env with your configuration
go run main.go
```

**Build custom image:**

```bash
docker build -t whosee-server .
```

## Version Tags

- `latest`: Latest stable release from main branch
- `v1.0.0`: Specific version release
- `main`: Latest commit from main branch
- `main-<sha>`: Specific commit from main branch

## Links

- **GitHub Repository**: https://github.com/AsisYu/whosee-server
- **Frontend Repository**: https://github.com/AsisYu/whosee-whois
- **Docker Hub**: https://hub.docker.com/r/hansomeyu/whosee-server
- **Documentation**: See GitHub repository docs/ folder

## Support

For issues, questions, or contributions:
- GitHub Issues: https://github.com/AsisYu/whosee-server/issues
- Documentation: https://github.com/AsisYu/whosee-server/tree/main/docs

## License

See LICENSE file in the GitHub repository.

---

**Note**: This service requires valid API keys from WHOIS data providers for full functionality. Free tier limitations may apply to third-party APIs.
