package web

import (
	"path"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/saltbo/gopkg/ginutil"
)

func SetupRoutes(ge *gin.Engine) {
	staticRouter := ge.Group("/")
	staticRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	staticRouter.Use(setStaticCacheControl())
	ginutil.SetupEmbedAssets(staticRouter, NewFS(), "/css", "/js", "/fonts")
	ge.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.RequestURI, "/api") {
			return
		}

		c.FileFromFS(c.Request.URL.Path, NewFS())
	})
}

func setStaticCacheControl() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isStaticAssetPath(c.Request.URL.Path) {
			c.Header("Cache-Control", "public, max-age=28800")
		}
		c.Next()
	}
}

func isStaticAssetPath(requestPath string) bool {
	if strings.HasPrefix(requestPath, "/css/") ||
		strings.HasPrefix(requestPath, "/js/") ||
		strings.HasPrefix(requestPath, "/fonts/") ||
		strings.HasPrefix(requestPath, "/img/") ||
		strings.HasPrefix(requestPath, "/images/") ||
		strings.HasPrefix(requestPath, "/assets/") {
		return true
	}

	switch strings.ToLower(path.Ext(requestPath)) {
	case ".css", ".js", ".mjs", ".map", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".eot", ".otf":
		return true
	default:
		return false
	}
}
