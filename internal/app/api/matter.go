package api

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/saltbo/gopkg/ginutil"
	"github.com/saltbo/zpan/internal/app/dao"
	"github.com/saltbo/zpan/internal/app/repo"
	"github.com/saltbo/zpan/internal/app/usecase/uploader"
	"github.com/saltbo/zpan/internal/app/usecase/vfs"
	"github.com/saltbo/zpan/internal/pkg/auth"
	"github.com/saltbo/zpan/internal/pkg/bind"
	"github.com/spf13/viper"
)

type FileResource struct {
	fs       vfs.VirtualFs
	up       uploader.Uploader
	userDao  *dao.User
}

func NewFileResource(fs vfs.VirtualFs, up uploader.Uploader) *FileResource {
	return &FileResource{
		fs:      fs,
		up:      up,
		userDao: dao.NewUser(),
	}
}

func (rs *FileResource) Register(router *gin.RouterGroup) {
	router.POST("/matters", rs.create)
	router.GET("/matters", rs.findAll)
	router.GET("/matters/:alias", rs.find)
	router.PATCH("/matters/:alias/done", rs.uploaded)
	router.PATCH("/matters/:alias/name", rs.rename)
	router.PATCH("/matters/:alias/location", rs.move)
	router.PATCH("/matters/:alias/duplicate", rs.copy)
	router.DELETE("/matters/:alias", rs.delete)
}

func (rs *FileResource) findAll(c *gin.Context) {
	p := new(bind.QueryFiles)
	if err := c.BindQuery(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	// Check if share_all_files is enabled in config
	var uid int64 = 0 // Default: show all users' files
	if !viper.GetBool("share.all_files") {
		uid = auth.UidGet(c) // Only show current user's files
	}

	opt := &repo.MatterListOption{
		QueryPage: repo.QueryPage(p.QueryPage),
		Sid:       p.Sid,
		Uid:       uid,
		Dir:       p.Dir,
		Type:      p.Type,
		Keyword:   p.Keyword,
	}
	list, total, err := rs.fs.List(c, opt)
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	// Add uploader username information to each matter
	userMap := make(map[int64]string)
	for _, m := range list {
		if _, exists := userMap[m.Uid]; !exists {
			// Get user info from database
			user, err := rs.userDao.Find(m.Uid)
			if err == nil && user != nil {
				userMap[m.Uid] = user.Username
			} else {
				userMap[m.Uid] = "Unknown"
			}
		}

		// Add username to Uploader field
		if m.Uploader == nil {
			m.Uploader = make(map[string]any)
		}
		m.Uploader["username"] = userMap[m.Uid]
	}

	ginutil.JSONList(c, list, total)
}

// create godoc
// @Tags Matters
// @Summary 创建文件
// @Description 创建文件
// @Accept json
// @Produce json
// @Security OAuth2Application[matter, admin]
// @Param body body bind.BodyMatter true "参数"
// @Success 200 {object} httputil.JSONResponse{data=entity.Matter}
// @Failure 400 {object} httputil.JSONResponse
// @Failure 500 {object} httputil.JSONResponse
// @Router /matters [post]
func (rs *FileResource) create(c *gin.Context) {
	p := new(bind.BodyMatter)
	if err := c.ShouldBindJSON(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	m := p.ToMatter(auth.UidGet(c))
	if err := rs.fs.Create(c, m); err != nil {
		// Distinguish between business logic errors and server errors
		// File already exists is a client error (400), not a server error (500)
		if strings.Contains(err.Error(), "cannot upload file with the same name") {
			ginutil.JSONBadRequest(c, err)
		} else {
			ginutil.JSONServerError(c, err)
		}
		return
	}

	ginutil.JSONData(c, m)
}

func (rs *FileResource) uploaded(c *gin.Context) {
	alias := c.Param("alias")
	m, err := rs.fs.Get(c, alias)
	if err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	if err := rs.up.UploadDone(c, m); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSONData(c, m)
}

func (rs *FileResource) find(c *gin.Context) {
	matter, err := rs.fs.Get(c, c.Param("alias"))
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSONData(c, matter)
}

func (rs *FileResource) rename(c *gin.Context) {
	p := new(bind.BodyFileRename)
	if err := c.ShouldBindJSON(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	if err := rs.fs.Rename(c, c.Param("alias"), p.NewName); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}

func (rs *FileResource) move(c *gin.Context) {
	p := new(bind.BodyFileMove)
	if err := c.ShouldBindJSON(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	if err := rs.fs.Move(c, c.Param("alias"), p.NewDir); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}

func (rs *FileResource) copy(c *gin.Context) {
	p := new(bind.BodyFileCopy)
	if err := c.ShouldBindJSON(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	m, err := rs.fs.Copy(c, c.Param("alias"), p.NewPath)
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSONData(c, m)
}

func (rs *FileResource) delete(c *gin.Context) {
	alias := c.Param("alias")
	
	// Get the matter to check ownership
	matter, err := rs.fs.Get(c, alias)
	if err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	// Check if user is the owner or is admin
	uid := auth.UidGet(c)
	if matter.Uid != uid && !auth.IsAdmin(c) {
		ginutil.JSONForbidden(c, fmt.Errorf("You can only delete files which uploaded by yourself"))
		return
	}

	if err := rs.fs.Delete(c, alias); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}
