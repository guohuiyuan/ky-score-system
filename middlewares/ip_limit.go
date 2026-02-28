package middlewares

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// IPRecord 记录 IP 的最后访问时间
type IPRecord struct {
	LastSubmit time.Time
}

var ipCache sync.Map

// 获取真实 IP（兼容 Cloudflare/Zeabur、Nginx 等多种代理环境）
func getClientIP(c *gin.Context) string {
	// 1. Cloudflare / Zeabur 环境：优先读取 CF-Connecting-IP
	if cfIP := c.GetHeader("CF-Connecting-IP"); cfIP != "" {
		return cfIP
	}

	// 2. 尝试从 X-Forwarded-For 获取
	xForwardedFor := c.GetHeader("X-Forwarded-For")
	if xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// 3. 尝试从 X-Real-IP 获取
	if xRealIP := c.GetHeader("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}

	// 4. 降级为 Gin 解析的值
	return c.ClientIP()
}

// SubmitLimit 限制同一 IP 的提交频率 (例如：60秒内只能提交一次)
func SubmitLimit(cooldownSeconds int) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := getClientIP(c)
		now := time.Now()

		// 查找该 IP 的记录
		if record, exists := ipCache.Load(clientIP); exists {
			lastSubmit := record.(IPRecord).LastSubmit
			elapsed := now.Sub(lastSubmit).Seconds()

			if elapsed < float64(cooldownSeconds) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "提交过于频繁，请稍后再试",
				})
				return
			}
		}

		// 更新该 IP 的最后提交时间
		ipCache.Store(clientIP, IPRecord{LastSubmit: now})

		// 定期清理过期的 IP 记录 (简单实现，避免内存泄漏)
		// 实际高并发生产环境中建议使用 Redis
		go cleanUpCache(cooldownSeconds)

		c.Next()
	}
}

// cleanUpCache 清理长时间未活跃的 IP 记录
func cleanUpCache(cooldownSeconds int) {
	now := time.Now()
	ipCache.Range(func(key, value interface{}) bool {
		record := value.(IPRecord)
		if now.Sub(record.LastSubmit).Seconds() > float64(cooldownSeconds*2) {
			ipCache.Delete(key)
		}
		return true
	})
}
