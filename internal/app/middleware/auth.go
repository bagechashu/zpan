package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/saltbo/gopkg/ginutil"

	"github.com/saltbo/zpan/internal/app/service"
	"github.com/saltbo/zpan/internal/pkg/auth"
)

func AuthMiddleware(c *gin.Context) {
	rc, err := token2Roles(c)
	if err != nil {
		ginutil.JSONUnauthorized(c, err)
		return
	}

	// 设置用户身份，权限检查由 OPA (authz.rego) 负责
	auth.UidSet(c, rc.Uid())
	auth.RoleSet(c, rc.Roles)
}

func token2Roles(c *gin.Context) (*service.RoleClaims, error) {
	const basicPrefix = "Basic "
	const BearerPrefix = "Bearer "
	cookieAuth := auth.TokenCookieGet(c)
	headerAuth := c.GetHeader("Authorization")
	if (cookieAuth == "" && headerAuth == "") || strings.HasPrefix(headerAuth, basicPrefix) {
		return service.NewRoleClaims("0", "anonymous", 3600, []string{"guest"}), nil
	}

	authToken := strings.TrimPrefix(headerAuth, BearerPrefix)
	if authToken == "" {
		authToken = cookieAuth
	}

	return service.NewToken().Verify(authToken)
}
