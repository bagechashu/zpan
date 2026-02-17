package api

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/saltbo/gopkg/ginutil"
	"github.com/saltbo/gopkg/jwtutil"
	"github.com/saltbo/gopkg/strutil"
	"github.com/saltbo/zpan/internal/app/repo"
	"github.com/saltbo/zpan/internal/app/usecase/vfs"
	"gorm.io/gorm"

	"github.com/saltbo/zpan/internal/app/dao"
	"github.com/saltbo/zpan/internal/app/entity"
	"github.com/saltbo/zpan/internal/app/model"
	"github.com/saltbo/zpan/internal/pkg/auth"
	"github.com/saltbo/zpan/internal/pkg/bind"
)

const ShareCookieTokenKey = "share-token"

type ShareResource struct {
	jwtutil.JWTUtil

	dShare  *dao.Share
	dMatter repo.Matter
	vfs     vfs.VirtualFs
}

func NewShareResource(dMatter repo.Matter, vfs vfs.VirtualFs) *ShareResource {
	return &ShareResource{
		dShare:  dao.NewShare(),
		dMatter: dMatter,
		vfs:     vfs,
	}
}

func (rs *ShareResource) Register(router *gin.RouterGroup) {
	router.GET("/shares/:alias", rs.find)
	router.GET("/shares", rs.findAll)
	router.POST("/shares", rs.create)
	router.PATCH("/shares/:alias", rs.update)
	router.DELETE("/shares/:alias", rs.delete)

	router.POST("/shares/:alias/token", rs.withdrawal)
	router.GET("/shares/:alias/matter", rs.findShareMatter)
	router.GET("/shares/:alias/matters", rs.findMatters)
	router.GET("/shares/:alias/matters/:mAlias", rs.findMatter)
}

func (rs *ShareResource) find(c *gin.Context) {
	share, err := rs.dShare.FindByAlias(c.Param("alias"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ginutil.JSONBadRequest(c, fmt.Errorf("share not found"))
		return
	} else if time.Now().After(share.ExpireAt) {
		ginutil.JSONForbidden(c, fmt.Errorf("share expired"))
		return
	}

	share.Secret = ""
	ginutil.JSONData(c, share)
}

func (rs *ShareResource) findAll(c *gin.Context) {
	p := new(bind.QueryPage)
	if err := c.BindQuery(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	list, total, err := rs.dShare.FindAll(auth.UidGet(c))
	if err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	ginutil.JSONList(c, list, total)
}

func (rs *ShareResource) create(c *gin.Context) {
	p := new(bind.BodyShare)
	if err := c.ShouldBindJSON(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	mMatter, err := rs.dMatter.FindByAlias(c, p.Matter)
	if err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	m := &model.Share{
		Alias:    strutil.RandomText(12),
		Uid:      auth.UidGet(c),
		Name:     mMatter.Name,
		Matter:   mMatter.Alias,
		Type:     mMatter.Type,
		ExpireAt: time.Now().Add(time.Second * time.Duration(p.ExpireSec)),
	}
	if p.Private {
		m.Secret = strutil.RandomText(5)
	}
	if err := rs.dShare.Create(m); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSONData(c, m)
}

func (rs *ShareResource) update(c *gin.Context) {
	p := new(bind.BodyShare)
	if err := c.ShouldBindJSON(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	share, err := rs.dShare.Find(p.Id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ginutil.JSONBadRequest(c, fmt.Errorf("share not found"))
		return
	}

	if p.Private && share.Secret == "" {
		share.Secret = strutil.RandomText(5)
	}

	if err := rs.dShare.Update(p.Id, share); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}

func (rs *ShareResource) delete(c *gin.Context) {
	share, err := rs.dShare.FindByAlias(c.Param("alias"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ginutil.JSONBadRequest(c, fmt.Errorf("share not exist"))
		return
	}

	if err := rs.dShare.Delete(share.Id); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}

func (rs *ShareResource) withdrawal(c *gin.Context) {
	p := new(bind.BodyShareDraw)
	if err := c.ShouldBindJSON(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	share, err := rs.dShare.FindByAlias(c.Param("alias"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ginutil.JSONBadRequest(c, fmt.Errorf("share not exist"))
		return
	} else if share.Secret != p.Secret {
		ginutil.JSONForbidden(c, fmt.Errorf("invalid secret"))
		return
	}

	claims := &jwt.StandardClaims{
		ExpiresAt: share.ExpireAt.Unix(),
		IssuedAt:  time.Now().Unix(),
		NotBefore: time.Now().Unix(),
		Subject:   share.Alias,
	}
	token, err := rs.JWTUtil.Issue(claims)
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	// 使用安全的 Cookie 设置，包含 HttpOnly、Secure 和 SameSite 属性
	auth.ShareCookieSet(c, ShareCookieTokenKey, token, int(time.Until(share.ExpireAt).Seconds()))
	ginutil.JSON(c)
}

func (rs *ShareResource) findShareMatter(c *gin.Context) {
	share, err := rs.dShare.FindByAlias(c.Param("alias"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ginutil.JSONBadRequest(c, fmt.Errorf("share not exist"))
		return
	}

	if err := rs.shareTokenVerify(c, share); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	mMatter, err := rs.vfs.Get(c, share.Matter)
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSONData(c, mMatter)
}

func (rs *ShareResource) findMatter(c *gin.Context) {
	share, err := rs.dShare.FindByAlias(c.Param("alias"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ginutil.JSONBadRequest(c, fmt.Errorf("share not exist"))
		return
	}

	if err := rs.shareTokenVerify(c, share); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	mMatter, err := rs.vfs.Get(c, c.Param("mAlias"))
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSONData(c, mMatter)
}

func (rs *ShareResource) findMatters(c *gin.Context) {
	p := new(bind.QueryShareMatters)
	if err := c.ShouldBind(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	share, err := rs.dShare.FindByAlias(c.Param("alias"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ginutil.JSONBadRequest(c, fmt.Errorf("share not exist"))
		return
	}

	if err := rs.shareTokenVerify(c, share); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	mMatter, err := rs.dMatter.FindByAlias(c, share.Matter)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ginutil.JSONBadRequest(c, fmt.Errorf("matter not found"))
		return
	}

	// p.Dir 已经是从前端传来的相对路径格式（如 "test/"），直接使用
	dir := p.Dir
	// 如果 dir 为空. 直接返回空 list, 避免误访问根目录下的所有文件
	if dir == "" {
		ginutil.JSONList(c, []*entity.Matter{}, 0)
		return
	}
	// 如果 dir 不以 "/" 结尾，添加 "/" 以确保它被正确识别为目录
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}

	list, total, err := rs.dMatter.FindAll(c, &repo.MatterListOption{
		QueryPage: repo.QueryPage{Offset: p.Offset, Limit: p.Limit},
		Uid:       mMatter.Uid,
		Dir:       dir,
	})
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	// 为每个 matter 填充 URL 以支持直接下载
	for _, m := range list {
		if !m.IsDir() {
			// 只为文件添加 URL，目录不需要
			if mWithUrl, err := rs.vfs.Get(c, m.Alias); err == nil {
				m.URL = mWithUrl.URL
			}
		}
	}

	ginutil.JSONList(c, list, total)
}

func (rs *ShareResource) shareTokenVerify(c *gin.Context, share *model.Share) error {
	if !share.Protected {
		return nil
	}

	tokenStr, err := c.Cookie(ShareCookieTokenKey)
	if err != nil {
		return err
	}

	if token, err := rs.JWTUtil.Parse(tokenStr, &jwt.StandardClaims{}); err != nil {
		return err
	} else if token.Claims.(*jwt.StandardClaims).Subject != share.Alias {
		return fmt.Errorf("unmatched token")
	}

	return nil
}
