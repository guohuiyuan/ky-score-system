package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminAuth 管理员鉴权中间件
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从 Cookie 中获取 token
		// 注意：这里的 "super_secret_token" 需要和 handlers/admin.go 中设置的一致
		token, err := c.Cookie("admin_token")
		
		if err != nil || token != "super_secret_token" {
			// 如果是 AJAX 请求 (比如 API 请求)，返回 JSON 错误
			if c.GetHeader("X-Requested-With") == "XMLHttpRequest" || c.ContentType() == "application/json" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权，请先登录"})
				return
			}
			
			// 如果是普通页面请求，重定向到登录页
			c.Redirect(http.StatusFound, "/ky/admin/login")
			c.Abort()
			return
		}

		// 鉴权通过，继续执行后续逻辑
		c.Next()
	}
}