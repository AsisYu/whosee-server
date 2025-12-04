/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-03-31 04:10:00
 * @Description: 认证中间件
 */

package middleware

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
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
// 用于JWT IP绑定验证，确保IP比较的准确性
func normalizeIP(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	// 解析IP地址
	parsed := net.ParseIP(trimmed)
	if parsed == nil {
		// 如果解析失败，返回原始值（可能包含端口或其他信息）
		return trimmed
	}

	// 如果是IPv4或IPv4映射的IPv6，返回IPv4格式
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}

	// 返回IPv6格式
	return parsed.String()
}

func AuthRequired(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Authorization头
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			log.Printf("Missing auth header from IP: %s", c.ClientIP())
			c.AbortWithStatusJSON(401, gin.H{"error": "Missing authorization header"})
			return
		}

		// 🔐 安全修复：验证Bearer前缀和长度，防止DoS攻击
		const bearerPrefix = "Bearer "
		if len(authHeader) < len(bearerPrefix) || !strings.HasPrefix(authHeader, bearerPrefix) {
			log.Printf("Invalid auth header format from IP: %s", c.ClientIP())
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid authorization header format"})
			return
		}

		// 安全提取token
		tokenString := strings.TrimSpace(authHeader[len(bearerPrefix):])
		if tokenString == "" {
			log.Printf("Empty token from IP: %s", c.ClientIP())
			c.AbortWithStatusJSON(401, gin.H{"error": "Empty token"})
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
			log.Printf("Token validation failed: %v", err)
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
			return
		}

		// 验证claims
		if claims, ok := token.Claims.(*Claims); ok && token.Valid {
			// 🔐 P2-1修复：验证JWT IP绑定
			// Token必须从其声明的IP地址使用，防止跨网络令牌重用
			requestIP := normalizeIP(c.ClientIP())
			tokenIP := normalizeIP(claims.IP)

			if requestIP == "" || tokenIP == "" || requestIP != tokenIP {
				log.Printf("[Security] Token IP mismatch: token_ip=%s request_ip=%s nonce=%s",
					claims.IP, c.ClientIP(), claims.Nonce)
				c.AbortWithStatusJSON(401, gin.H{
					"error": "Token IP mismatch",
					"code":  "IP_BINDING_FAILED",
				})
				return
			}

			// 验证nonce是否已使用
			nonceKey := fmt.Sprintf("nonce:%s", claims.Nonce)
			if exists, _ := rdb.Exists(c, nonceKey).Result(); exists == 1 {
				c.AbortWithStatusJSON(401, gin.H{"error": "Token already used"})
				return
			}

			// 记录nonce
			rdb.Set(c, nonceKey, true, TokenExpiration)

			c.Next()
		} else {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token claims"})
		}
	}
}

// 生成临时Token的处理函数
func GenerateToken(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// 检查IP的token请求频率
		key := fmt.Sprintf("token:ip:%s", clientIP)
		count, _ := rdb.Incr(c, key).Result()
		rdb.Expire(c, key, time.Minute)

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
			IP:    clientIP,
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
