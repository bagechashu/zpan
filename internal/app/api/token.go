package api

import (
	"github.com/gin-gonic/gin"
	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	"github.com/saltbo/gopkg/ginutil"
	_ "github.com/saltbo/gopkg/httputil"
	"github.com/spf13/viper"

	"github.com/saltbo/zpan/internal/app/service"
	"github.com/saltbo/zpan/internal/pkg/auth"
	"github.com/saltbo/zpan/internal/pkg/bind"
	"github.com/saltbo/zpan/internal/pkg/logger"
)

type TokenResource struct {
	sUser *service.User

	srv *server.Server
}

func NewTokenResource() *TokenResource {
	uk := service.NewUserKey()
	uk.LoadExistClient()
	manager := manage.NewManager()
	manager.MapAccessGenerate(uk)
	manager.MapClientStorage(uk.ClientStore())
	manager.MustTokenStorage(store.NewMemoryTokenStore())

	srv := server.NewDefaultServer(manager)
	srv.SetAllowGetAccessRequest(true)
	srv.SetClientInfoHandler(server.ClientBasicHandler)
	srv.SetInternalErrorHandler(func(err error) (re *errors.Response) {
		logger.Error("Internal Error:", "error", err)
		return
	})

	return &TokenResource{
		srv:   srv,
		sUser: service.NewUser(),
	}
}

func (rs *TokenResource) Register(router *gin.RouterGroup) {
	router.POST("/tokens", rs.create)
	router.DELETE("/tokens", rs.delete)
}

// create godoc
// @Tags Tokens
// @Summary 登录/密码重置
// @Description 用于账户登录和申请密码重置
// @Accept json
// @Produce json
// @Param body body bind.BodyToken true "参数"
// @Success 200 {object} httputil.JSONResponse
// @Failure 400 {object} httputil.JSONResponse
// @Failure 500 {object} httputil.JSONResponse
// @Router /tokens [post]
func (rs *TokenResource) create(c *gin.Context) {
	// support gen oauth2 access_token
	if _, _, ok := c.Request.BasicAuth(); ok {
		rs.srv.HandleTokenRequest(c.Writer, c.Request)
		return
	}

	p := new(bind.BodyToken)
	if err := c.ShouldBindJSON(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	// issue a recover token to the user mail
	if p.Password == "" {
		if err := rs.sUser.PasswordResetApply(ginutil.GetOrigin(c), p.Email); err != nil {
			ginutil.JSONServerError(c, err)
			return
		}

		ginutil.JSON(c)
		return
	}

	// issue a signIn token into cookies
	// ⚠️ 使用配置中的 TTL，默认 15 分钟用于访问令牌
	// 实际的登录会话可以通过 refresh token（使用更长的 TTL）来维护
	accessTokenTTL := viper.GetInt("jwt.access_token_ttl")
	if accessTokenTTL <= 0 {
		accessTokenTTL = 15 * 60 // 默认 15 分钟
	}
	
	user, err := rs.sUser.SignIn(p.Email, p.Password, accessTokenTTL)
	if err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	auth.TokenCookieSet(c, user.Token, accessTokenTTL)
	// 返回 token 和用户信息，前端可将 token 存储在 localStorage
	c.JSON(200, gin.H{
		"token":    user.Token,
		"uid":      user.Id,
		"username": user.Username,
		"roles":    user.Roles,
	})
}

// delete godoc
// @Tags Tokens
// @Summary 退出登录
// @Description 用户状态登出
// @Accept json
// @Produce json
// @Success 200 {object} httputil.JSONResponse
// @Failure 400 {object} httputil.JSONResponse
// @Failure 500 {object} httputil.JSONResponse
// @Router /tokens [delete]
func (rs *TokenResource) delete(c *gin.Context) {
	// 清除认证令牌 Cookie
	auth.TokenCookieSet(c, "", -1)
	ginutil.JSON(c)
}
