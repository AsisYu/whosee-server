package middleware

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

func Monitoring(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		clientIP := c.ClientIP()

		// 检查IP黑名单
		blacklistKey := fmt.Sprintf("blacklist:%s", clientIP)
		if exists, _ := rdb.Exists(c, blacklistKey).Result(); exists == 1 {
			c.AbortWithStatusJSON(403, gin.H{
				"error": "IP已被封禁",
				"code":  "IP_BANNED",
			})
			return
		}

		c.Next()

		// 记录并检查QPS
		qpsKey := fmt.Sprintf("qps:%s:%s", time.Now().Format("2006-01-02-15-04"), c.Request.URL.Path)
		qps, _ := rdb.Incr(c, qpsKey).Result()
		rdb.Expire(c, qpsKey, time.Minute)

		if qps > 1000 {
			log.Printf("[ALERT] High QPS detected: %d for %s from IP: %s", qps, c.Request.URL.Path, clientIP)
			// 自动封禁可疑IP
			if qps > 2000 {
				rdb.Set(c, blacklistKey, true, 24*time.Hour)
				log.Printf("[SECURITY] IP banned for high QPS: %s", clientIP)
			}
		}

		// 记录响应时间
		duration := time.Since(start)
		if duration > 5*time.Second {
			key := fmt.Sprintf("slow:req:%s", time.Now().Format("2006-01-02"))
			rdb.ZIncrBy(c, key, 1, c.Request.URL.Path)
			log.Printf("[ALERT] Slow request detected: %v for %s from IP: %s", duration, c.Request.URL.Path, clientIP)
		}

		// 记录错误率
		if c.Writer.Status() >= 400 {
			key := fmt.Sprintf("errors:%s", time.Now().Format("2006-01-02"))
			rdb.Incr(c, key)
			// 记录4xx错误的IP
			if c.Writer.Status() >= 400 && c.Writer.Status() < 500 {
				key = fmt.Sprintf("suspicious:ip:%s", time.Now().Format("2006-01-02"))
				rdb.ZIncrBy(c, key, 1, clientIP)
			}
		}
	}
}
