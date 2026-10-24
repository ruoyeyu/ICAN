package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/handler"
	"net/http"
	"sync"
	"time"
)

// A process-wide admission ceiling cannot be bypassed with forged proxy headers.
// Value is supplied by the operator; there is no silently chosen production cap.
func AuthRateLimit() gin.HandlerFunc {
	var mu sync.Mutex
	var minute int64
	var attempts int
	return func(c *gin.Context) {
		switch c.Request.URL.Path {
		case "/api/auth/login", "/api/auth/register", "/api/admin/login", "/api/auth/password":
			if c.Request.Method != http.MethodPost {
				c.Next()
				return
			}
			mu.Lock()
			now := time.Now().Unix() / 60
			if now != minute {
				minute = now
				attempts = 0
			}
			allowed := config.Cfg.AuthAttemptsPerMinute > 0 && attempts < config.Cfg.AuthAttemptsPerMinute
			if allowed {
				attempts++
			}
			mu.Unlock()
			if !allowed {
				handler.FailWithStatus(c.Writer, http.StatusTooManyRequests, "登录请求受限，请稍后重试或联系管理员配置登录保护")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
