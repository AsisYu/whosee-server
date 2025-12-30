/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-03-31 04:10:00
 * @Description: 认证中间件
 */

package middleware

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"whosee/pkg/logger"
)

const (
	TokenExpiration = 30 * time.Second
)

type Claims struct {
	jwt.StandardClaims
	Nonce string `json:"nonce"`
	IP    string `json:"ip"`
}

// normalizeIP 规范化IP地址，处理IPv4和IPv6映射
// 关键改进：统一所有localhost地址为127.0.0.1，解决::1和127.0.0.1不匹配问题
func normalizeIP(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	// 移除端口号（如果存在）
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		trimmed = host
	}

	// 移除IPv6地址的方括号
	trimmed = strings.Trim(trimmed, "[]")

	// 解析IP地址
	parsed := net.ParseIP(trimmed)
	if parsed == nil {
		// 如果解析失败，返回原始值
		return trimmed
	}

	// 统一所有loopback地址（::1, 127.0.0.1, ::ffff:127.0.0.1）为127.0.0.1
	// 这解决了开发环境中IPv4/IPv6 localhost不匹配的问题
	if parsed.IsLoopback() {
		return "127.0.0.1"
	}

	// 如果是IPv4或IPv4映射的IPv6，返回IPv4格式
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}

	// 返回规范化的IPv6格式
	return parsed.String()
}

// respondAuthError 统一的认证错误响应
// 开发模式：返回详细错误信息帮助调试
// 生产模式：只返回安全的错误代码，防止信息泄露
func respondAuthError(c *gin.Context, status int, publicMsg, code, detail string) {
	payload := gin.H{"error": publicMsg}
	if code != "" {
		payload["code"] = code
	}
	// 开发模式下返回详细信息，帮助前端开发调试
	if gin.Mode() != gin.ReleaseMode && detail != "" {
		payload["detail"] = detail
		payload["hint"] = "This detail is only shown in development mode"
	}
	c.AbortWithStatusJSON(status, payload)
}

func AuthRequired(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取带request_id的logger
		log := logger.WithRequest(c, "Auth")

		// 获取Authorization头
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			log.Warnf("Missing auth header")
			respondAuthError(c, 401, "Missing authorization header", "MISSING_AUTH_HEADER", "")
			return
		}

		// 🔐 安全修复：验证Bearer前缀和长度，防止DoS攻击
		const bearerPrefix = "Bearer "
		if len(authHeader) < len(bearerPrefix) || !strings.HasPrefix(authHeader, bearerPrefix) {
			log.Warnf("Invalid auth header format")
			respondAuthError(c, 401, "Invalid authorization header format", "INVALID_AUTH_FORMAT", "")
			return
		}

		// 安全提取token
		tokenString := strings.TrimSpace(authHeader[len(bearerPrefix):])
		if tokenString == "" {
			log.Warnf("Empty token")
			respondAuthError(c, 401, "Empty token", "EMPTY_TOKEN", "")
			return
		}

		// 验证JWT
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			log.Errorf("Token parse failed: %v", err)
			respondAuthError(c, 401, "Invalid token", "TOKEN_PARSE_FAILED", err.Error())
			return
		}

		// 验证claims
		if claims, ok := token.Claims.(*Claims); ok && token.Valid {
			// 🔐 P2-1修复：验证JWT IP绑定
			// Token必须从其声明的IP地址使用，防止跨网络令牌重用
			requestIP := normalizeIP(c.ClientIP())
			tokenIP := normalizeIP(claims.IP)

			if requestIP == "" || tokenIP == "" || requestIP != tokenIP {
				detail := fmt.Sprintf("token_ip=%s request_ip=%s (normalized: token=%s request=%s) nonce=%s",
					claims.IP, c.ClientIP(), tokenIP, requestIP, claims.Nonce)
				log.With("token_ip", tokenIP, "request_ip", requestIP, "nonce", claims.Nonce).
					Warnf("Token IP mismatch: token bound to %s but used from %s", tokenIP, requestIP)
				respondAuthError(c, 401, "Invalid token", "IP_BINDING_FAILED", detail)
				return
			}

			// 🔐 安全修复：使用SetNX原子操作防止nonce重放竞争条件
			// SetNX是原子操作，只有第一个请求能成功设置nonce，后续请求会失败
			nonceKey := fmt.Sprintf("nonce:%s", claims.Nonce)
			nonceStored, err := rdb.SetNX(c, nonceKey, true, TokenExpiration).Result()
			if err != nil {
				log.Errorf("Redis error recording nonce: %v", err)
				respondAuthError(c, 500, "Internal server error", "NONCE_CHECK_FAILED", fmt.Sprintf("Redis error: %v", err))
				return
			}
			if !nonceStored {
				log.With("nonce", claims.Nonce).Warnf("Nonce replay attack detected")
				respondAuthError(c, 401, "Invalid token", "NONCE_REPLAY", fmt.Sprintf("nonce=%s already used", claims.Nonce))
				return
			}

			c.Next()
		} else {
			log.Warnf("Invalid token claims")
			respondAuthError(c, 401, "Invalid token", "INVALID_CLAIMS", "token claims validation failed")
		}
	}
}

// 生成临时Token的处理函数
func GenerateToken(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取带request_id的logger
		log := logger.WithRequest(c, "Auth")

		// 规范化IP地址（关键修复：确保token中的IP与后续验证时使用的IP格式一致）
		clientIP := normalizeIP(c.ClientIP())

		// 🔐 安全修复：Rate limiter fail-closed - Redis错误时拒绝请求而非允许通过
		key := fmt.Sprintf("token:ip:%s", clientIP)
		count, err := rdb.Incr(c, key).Result()
		if err != nil {
			log.Errorf("Redis error incrementing token rate limiter: %v", err)
			c.JSON(503, gin.H{"error": "Rate limiter unavailable", "code": "RATE_LIMITER_UNAVAILABLE"})
			return
		}
		if err := rdb.Expire(c, key, time.Minute).Err(); err != nil {
			log.Errorf("Redis error setting token rate limiter TTL: %v", err)
			c.JSON(503, gin.H{"error": "Rate limiter unavailable", "code": "RATE_LIMITER_UNAVAILABLE"})
			return
		}

		if count > 30 { // 每分钟最多30个token
			c.JSON(429, gin.H{
				"error": "请求过于频繁",
				"code":  "TOO_MANY_REQUESTS",
			})
			return
		}

		nonce := fmt.Sprintf("%d", time.Now().UnixNano())
		claims := Claims{
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(TokenExpiration).Unix(),
				IssuedAt:  time.Now().Unix(),
				Issuer:    "whois-api.os.tn",
			},
			Nonce: nonce,
			IP:    clientIP,  // 使用规范化后的IP
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signedToken, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Failed to generate token",
				"code":  "TOKEN_GENERATION_FAILED",
			})
			return
		}

		c.JSON(200, gin.H{"token": signedToken})
	}
}
