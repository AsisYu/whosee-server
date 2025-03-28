package middleware

import (
	"fmt"
	"log"
	"os"
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

func AuthRequired(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			log.Printf("Missing auth header from IP: %s", c.ClientIP())
			c.AbortWithStatusJSON(401, gin.H{"error": "Missing authorization header"})
			return
		}

		// 验证JWT格式
		tokenString := authHeader[7:] // 移除"Bearer "前缀
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
