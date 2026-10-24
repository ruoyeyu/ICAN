package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/handler"
)

func SecurityBoundary(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Cache-Control", "no-store")
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
		origin := c.GetHeader("Origin")
		if (origin != "" && origin != config.Cfg.PublicOrigin) || c.GetHeader("Sec-Fetch-Site") == "cross-site" {
			handler.FailWithStatus(c.Writer, http.StatusForbidden, "请求来源不匹配")
			c.Abort()
			return
		}
	}
	path := c.Request.URL.Path
	for _, fragment := range []string{"/agent-skills", "/comfy-bridges", "/bridge/", "/workflow", "/ai/autodl/", "/auth/linux-do/", "/anonymous/", "/credit-logs", "/prompt-categories/sync"} {
		if strings.Contains(path, fragment) {
			handler.FailWithStatus(c.Writer, http.StatusNotFound, "当前部署未启用此功能")
			c.Abort()
			return
		}
	}
	if strings.HasSuffix(path, "/credits") || strings.HasSuffix(path, "/files/direct") || strings.HasSuffix(path, "/user-config/storage") {
		handler.FailWithStatus(c.Writer, http.StatusNotFound, "当前部署未启用此功能")
		c.Abort()
		return
	}
	c.Next()
}
