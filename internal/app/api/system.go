package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/saltbo/gopkg/ginutil"
	"github.com/saltbo/gopkg/jwtutil"
	"github.com/saltbo/gopkg/strutil"
	"github.com/saltbo/zpan/internal/app/entity"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"

	"github.com/saltbo/zpan/internal/app/dao"
	"github.com/saltbo/zpan/internal/app/middleware"
	"github.com/saltbo/zpan/internal/app/model"
	"github.com/saltbo/zpan/internal/app/service"
	"github.com/saltbo/zpan/internal/pkg/bind"
	"github.com/saltbo/zpan/internal/pkg/provider"
)

type Option struct {
	jwtutil.JWTUtil

	sOption *service.Option
}

func NewOptionResource() *Option {
	return &Option{
		sOption: service.NewOption(),
	}
}

func (rs *Option) Register(router *gin.RouterGroup) {
	router.Use(middleware.AuthMiddleware)
	router.PUT("/system/database", rs.setupDatabase)
	router.PUT("/system/account", rs.createAdministrator)
	router.GET("/system/providers", rs.providers)
	router.GET("/system/matter-path-envs", rs.matterPathEnvs)
	router.GET("/system/options/:name", rs.find)
	router.PUT("/system/options/:name", rs.update)
}

func (rs *Option) setupDatabase(c *gin.Context) {
	if viper.IsSet("installed") {
		ginutil.JSONBadRequest(c, fmt.Errorf("datebase config already installed"))
		return
	}

	p := make(map[string]string)
	if err := c.ShouldBind(&p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	if err := dao.Init(p["driver"], p["dsn"]); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	if err := dao.NewOption().Init(); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	viper.Set("database.driver", p["driver"])
	viper.Set("database.dsn", p["dsn"])
	cfgFile := viper.ConfigFileUsed()
	if cfgFile == "" {
		cfgFile = "/etc/zpan/config.yml"
	}
	if err := viper.WriteConfigAs(cfgFile); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}

func (rs *Option) createAdministrator(c *gin.Context) {
	if viper.IsSet("installed") {
		ginutil.JSONBadRequest(c, fmt.Errorf("datebase config already installed"))
		return
	}

	p := new(bind.BodyUserCreation)
	if err := c.ShouldBind(&p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}
	// 创建基本信息
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	user := &model.User{
		Email:    p.Email,
		Username: "admin",
		Password: string(hashedPwd),
		Roles:    "admin",
		Ticket:   strutil.RandomText(6),
		Status:   model.StatusActivated,
	}
	if _, err := dao.NewUser().Create(user, 0); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	// 设置已安装完成的标志
	viper.Set("installed", true)
	ginutil.JSON(c)

}

func (rs *Option) providers(c *gin.Context) {
	ginutil.JSONData(c, provider.GetProviders())
}

func (rs *Option) find(c *gin.Context) {
	ret, err := dao.NewOption().Get(c.Param("name"))
	if err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	ginutil.JSONData(c, ret)
}

func (rs *Option) update(c *gin.Context) {
	p := make(map[string]interface{})
	if err := c.ShouldBind(&p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	if err := rs.sOption.Update(c.Param("name"), p); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}

func (rs *Option) matterPathEnvs(c *gin.Context) {
	ginutil.JSONData(c, entity.SupportEnvs)
}
