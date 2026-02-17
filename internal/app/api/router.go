package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/saltbo/gopkg/ginutil"
	"github.com/saltbo/zpan/internal/app/middleware"
	_ "github.com/saltbo/zpan/internal/docs"
	"github.com/saltbo/zpan/internal/pkg/logger"
)

// @title zpan
// @description zpan apis
// @version 1.0.0

// @BasePath /api/
// @securitydefinitions.oauth2.application OAuth2Application
// @scope.matter Grants matter access and write
// @scope.admin Grants read and write access to administrative information
// @tokenUrl /api/tokens
// @name Authorization

// @contact.name API Support
// @contact.url http://zpan.space
// @contact.email saltbo@foxmail.com

// @license.name GPL 3.0
// @license.url https://github.com/saltbo/zpan/blob/master/LICENSE

func SetupRoutes(ge *gin.Engine, repository *Repository) {
	if logger.GetLogLevel() <= slog.LevelDebug {
		ginutil.SetupSwagger(ge)
	}

	apiRouter := ge.Group("/api")
	apiRouter.Use(middleware.AuthMiddleware) // 认证中间件：验证 token 并设置用户身份
	apiRouter.Use(middleware.OpaMiddleware)  // 授权中间件：检查权限
	ginutil.SetupResource(apiRouter,
		repository.option,
		repository.file,
		repository.storage,
		repository.share,
		repository.token,
		repository.user,
		repository.userKey,
		repository.recycleBin,
	)
}
