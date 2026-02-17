package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// CorsMiddleware 处理跨域请求（CORS）
// 配置项举例：
//
//	cors:
//	  enabled: true
//	  allow_origins:
//	    - http://localhost:3000
//	    - https://example.com
//	  allow_methods:
//	    - GET
//	    - POST
//	    - PUT
//	    - DELETE
//	    - OPTIONS
//	    - PATCH
//	  allow_headers:
//	    - Content-Type
//	    - Authorization
//	  allow_credentials: true
//	  max_age: 3600
func CorsMiddleware(c *gin.Context) {
	// 检查 CORS 是否启用（默认禁用）
	if !viper.GetBool("cors.enabled") {
		c.Next()
		return
	}

	origin := c.Request.Header.Get("Origin")

	// 检查 Origin 是否被允许
	allowOrigins := viper.GetStringSlice("cors.allow_origins")
	if len(allowOrigins) == 0 {
		// 如果未配置 allow_origins，默认不允许CORS
		c.Next()
		return
	}

	allowed := false
	if len(allowOrigins) > 0 && allowOrigins[0] == "*" {
		// 通配符模式：允许所有源（不推荐在生产环境使用）
		allowed = true
	} else {
		// 精确匹配检查
		for _, o := range allowOrigins {
			if o == origin {
				allowed = true
				break
			}
		}
	}

	if allowed {
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Methods", getConfigValue("cors.allow_methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH"))
		c.Writer.Header().Set("Access-Control-Allow-Headers", getConfigValue("cors.allow_headers", "Content-Type, Authorization"))

		if viper.GetBool("cors.allow_credentials") {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		maxAge := viper.GetInt("cors.max_age")
		if maxAge > 0 {
			c.Writer.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", maxAge))
		}
	}

	// 处理 OPTIONS 预检请求
	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(204)
		return
	}

	c.Next()
}

func getConfigValue(key, defaultValue string) string {
	if val := viper.GetString(key); val != "" {
		return val
	}
	return defaultValue
}
