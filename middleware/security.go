/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-01-17 23:47:06
 * @Description: 增强的Web安全中间件
 */
package middleware

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityConfig 安全中间件配置
type SecurityConfig struct {
	EnableCSP            bool     // 是否启用内容安全策略
	EnableHSTS           bool     // 是否启用HSTS
	CSPSources           []string // CSP允许的源
	FrameOptions         string   // X-Frame-Options 设置
	XSSProtection        string   // X-XSS-Protection 设置
	ContentTypeOptions   string   // X-Content-Type-Options 设置
	ReferrerPolicy       string   // Referrer-Policy 设置
	PermissionsPolicy    string   // Permissions-Policy 设置
	HSTSMaxAge           int      // HSTS的最大有效期（秒）
	HSTSIncludeSubDomains bool     // HSTS是否包括子域名
	HSTSPreload          bool     // HSTS是否预加载
}

// DefaultSecurityConfig 返回默认的安全配置
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		EnableCSP:            true,
		EnableHSTS:           true,
		CSPSources:           []string{"'self'"},
		FrameOptions:         "DENY",
		XSSProtection:        "1; mode=block",
		ContentTypeOptions:   "nosniff",
		ReferrerPolicy:       "strict-origin-when-cross-origin",
		PermissionsPolicy:    "geolocation=(), microphone=(), camera=()",
		HSTSMaxAge:           31536000, // 1年
		HSTSIncludeSubDomains: true,
		HSTSPreload:          false,
	}
}

// Security 标准安全中间件
func Security() gin.HandlerFunc {
	return SecurityWithConfig(DefaultSecurityConfig())
}

// SecurityWithConfig 带配置的安全中间件
func SecurityWithConfig(config SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// X-Content-Type-Options 防止MIME类型嗅探
		c.Header("X-Content-Type-Options", config.ContentTypeOptions)

		// X-Frame-Options 防止点击劫持
		c.Header("X-Frame-Options", config.FrameOptions)

		// X-XSS-Protection 启用XSS过滤
		c.Header("X-XSS-Protection", config.XSSProtection)

		// CSP 内容安全策略
		if config.EnableCSP {
			// 从环境变量获取CSP配置 (如果有)
			cspSources := config.CSPSources
			if envCSP := os.Getenv("CSP_SOURCES"); envCSP != "" {
				cspSources = strings.Split(envCSP, ",")
			}

			// 构建CSP策略
			csp := "default-src " + strings.Join(cspSources, " ")
			
			// 添加其他CSP指令
			csp += "; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'"
			c.Header("Content-Security-Policy", csp)
		}

		// HSTS 强制使用HTTPS
		if config.EnableHSTS {
			hstsValue := "max-age=" + string(rune(config.HSTSMaxAge))
			if config.HSTSIncludeSubDomains {
				hstsValue += "; includeSubDomains"
			}
			if config.HSTSPreload {
				hstsValue += "; preload"
			}
			c.Header("Strict-Transport-Security", hstsValue)
		}

		// Referrer-Policy 控制引用来源信息
		c.Header("Referrer-Policy", config.ReferrerPolicy)

		// Permissions-Policy 控制浏览器功能
		c.Header("Permissions-Policy", config.PermissionsPolicy)

		// 处理OPTIONS请求 (CORS预检)
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
