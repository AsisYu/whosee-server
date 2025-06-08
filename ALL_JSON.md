# Whosee-Whois API 返回格式文档

本文档列出了所有API端点的返回JSON格式，供前端开发参考。

## 目录

- [认证相关 API](#认证相关-api)
  - [获取JWT令牌](#获取jwt令牌)
- [WHOIS查询 API](#whois查询-api)
- [DNS查询 API](#dns查询-api)
- [网站截图 API](#网站截图-api)
  - [普通截图](#普通截图)
  - [Base64编码截图](#base64编码截图)
- [ITDog测速 API](#itdog测速-api)
  - [普通截图](#普通截图-1)
  - [Base64编码截图](#base64编码截图-1)
- [健康检查 API](#健康检查-api)

## 认证相关 API

### 获取JWT令牌

**端点**: `/api/auth/token`  
**方法**: POST  
**返回格式**:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresAt": "2025-05-10T17:50:46+08:00"
}
```

## WHOIS查询 API

**端点**: `/api/v1/whois` 或 `/api/v1/whois/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "available": false,
    "domain": "example.com",
    "registrar": "Example Registrar, LLC",
    "creationDate": "1995-08-14T04:00:00Z",
    "expiryDate": "2025-08-13T04:00:00Z",
    "status": ["clientDeleteProhibited", "clientTransferProhibited", "clientUpdateProhibited"],
    "nameServers": ["ns1.example.com", "ns2.example.com"],
    "updatedDate": "2023-08-14T04:00:00Z",
    "statusCode": 200,
    "statusMessage": "Domain found",
    "sourceProvider": "whois-provider-name"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "cached": true,
    "cachedAt": "2025-05-10T16:30:00+08:00",
    "processing": 25
  }
}
```

## DNS查询 API

**端点**: `/api/v1/dns` 或 `/api/v1/dns/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "domain": "example.com",
    "records": {
      "A": [
        {
          "name": "example.com",
          "ttl": 3600,
          "value": "93.184.216.34"
        }
      ],
      "MX": [
        {
          "name": "example.com",
          "ttl": 3600,
          "priority": 10,
          "value": "mail.example.com"
        }
      ],
      "NS": [
        {
          "name": "example.com",
          "ttl": 172800,
          "value": "ns1.example.com"
        },
        {
          "name": "example.com",
          "ttl": 172800,
          "value": "ns2.example.com"
        }
      ],
      "TXT": [
        {
          "name": "example.com",
          "ttl": 3600,
          "value": "v=spf1 include:example.com ~all"
        }
      ]
    },
    "status": "success"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "processing": 150
  }
}
```

## 网站截图 API

### 普通截图

**端点**: `/api/v1/screenshot` 或 `/api/v1/screenshot/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "domain": "example.com",
    "imageUrl": "/static/screenshots/example.com_20250510173000.png",
    "status": "success",
    "title": "Example Domain",
    "timestamp": "2025-05-10T17:30:00+08:00"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "processing": 3500
  }
}
```

### Base64编码截图

**端点**: `/api/v1/screenshot/base64/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "success": true,
  "domain": "example.com",
  "imageData": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...",
  "title": "Example Domain",
  "timestamp": "2025-05-10T17:30:00+08:00",
  "processingTime": 3200
}
```

## ITDog测速 API

### 普通截图

**端点**: `/api/v1/itdog/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "domain": "example.com",
    "imageUrl": "/static/itdog/example.com_20250510173000.png",
    "status": "success",
    "testResults": {
      "pingMin": 35.6,
      "pingAvg": 42.8,
      "pingMax": 55.3,
      "pingLoss": 0,
      "speedRank": "A",
      "testLocations": ["北京", "上海", "广州", "深圳"]
    },
    "timestamp": "2025-05-10T17:30:00+08:00"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "processing": 5000
  }
}
```

### Base64编码截图

**端点**: `/api/v1/itdog/base64/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "success": true,
  "domain": "example.com",
  "imageData": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...",
  "testResults": {
    "pingMin": 35.6,
    "pingAvg": 42.8,
    "pingMax": 55.3,
    "pingLoss": 0,
    "speedRank": "A",
    "testLocations": ["北京", "上海", "广州", "深圳"]
  },
  "timestamp": "2025-05-10T17:30:00+08:00",
  "processingTime": 4800
}
```

### 表格截图

**端点**: `/api/v1/itdog/table/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "domain": "example.com",
    "imageUrl": "/static/itdog/table/example.com_20250510173000.png",
    "status": "success",
    "timestamp": "2025-05-10T17:30:00+08:00"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "processing": 4500
  }
}
```

### 表格Base64截图

**端点**: `/api/v1/itdog/table/base64/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "domain": "example.com",
    "imageBase64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...",
    "status": "success",
    "timestamp": "2025-05-10T17:30:00+08:00"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "processing": 4500
  }
}
```

### IP统计截图

**端点**: `/api/v1/itdog/ip/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "domain": "example.com",
    "imageUrl": "/static/itdog/ip/example.com_20250510173000.png",
    "status": "success",
    "resolveInfo": {
      "ipCount": 1,
      "primaryIp": "158.179.173.57",
      "ipDistribution": [{
        "ip": "158.179.173.57",
        "percentage": 100.0
      }]
    },
    "timestamp": "2025-05-10T17:30:00+08:00"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "processing": 3800
  }
}
```

### IP统计Base64截图

**端点**: `/api/v1/itdog/ip/base64/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "domain": "example.com",
    "imageBase64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...",
    "status": "success",
    "resolveInfo": {
      "ipCount": 1,
      "primaryIp": "158.179.173.57",
      "ipDistribution": [{
        "ip": "158.179.173.57",
        "percentage": 100.0
      }]
    },
    "timestamp": "2025-05-10T17:30:00+08:00"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "processing": 3800
  }
}
```

### 全国解析截图

**端点**: `/api/v1/itdog/resolve/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "domain": "example.com",
    "imageUrl": "/static/itdog/resolve/example.com_20250510173000.png",
    "status": "success",
    "resolveInfo": {
      "ipCount": 1,
      "primaryIp": "158.179.173.57",
      "regionalData": {
        "北京": "158.179.173.57",
        "上海": "158.179.173.57",
        "广州": "158.179.173.57",
        "深圳": "158.179.173.57"
      }
    },
    "timestamp": "2025-05-10T17:30:00+08:00"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "processing": 6000
  }
}
```

### 全国解析Base64截图

**端点**: `/api/v1/itdog/resolve/base64/:domain`  
**方法**: GET  
**返回格式**:

```json
{
  "data": {
    "domain": "example.com",
    "imageBase64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...",
    "status": "success",
    "resolveInfo": {
      "ipCount": 1,
      "primaryIp": "158.179.173.57",
      "regionalData": {
        "北京": "158.179.173.57",
        "上海": "158.179.173.57",
        "广州": "158.179.173.57",
        "深圳": "158.179.173.57"
      }
    },
    "timestamp": "2025-05-10T17:30:00+08:00"
  },
  "meta": {
    "timestamp": "2025-05-10T17:30:00+08:00",
    "processing": 6000
  }
}
```

## 健康检查 API

**端点**: `/api/health`  
**方法**: GET  
**参数**: `detailed=true|false` (可选，默认false)  
**返回格式**:

```json
{
  "status": "up",
  "version": "1.1.0",
  "time": "2025-05-10T09:30:00Z",
  "services": {
    "redis": {
      "status": "up",
      "latency": 2.5,
      "lastCheck": "2025-05-10T09:29:50Z"
    },
    "dns": {
      "status": "up",
      "provider": "system",
      "lastCheck": "2025-05-10T09:29:50Z"
    },
    "whois": {
      "status": "up",
      "providers": ["whoisfreaks", "whoisxml"],
      "lastCheck": "2025-05-10T09:29:50Z"
    }
  },
  "lastCheck": "2025-05-10T09:30:00Z"
}
```

## 错误响应格式

所有API在发生错误时都会返回一致的错误格式：

```json
{
  "error": "ERROR_CODE",
  "message": "详细的错误描述信息",
  "timestamp": "2025-05-10T17:30:00+08:00",
  "path": "/api/v1/whois/invalid-domain"
}
```

常见错误代码：

- `INVALID_DOMAIN`: 域名格式无效
- `QUERY_ERROR`: 查询过程中发生错误
- `SERVICE_BUSY`: 服务忙碌，请稍后重试
- `TIMEOUT`: 请求超时
- `SERVICE_UNAVAILABLE`: 服务不可用
- `UNAUTHORIZED`: 未授权访问
- `FORBIDDEN`: 禁止访问
- `REQUEST_ENTITY_TOO_LARGE`: 请求实体过大