package api

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/saltbo/gopkg/ginutil"
	"github.com/saltbo/gopkg/jwtutil"
	"github.com/saltbo/zpan/internal/app/entity"
	"github.com/saltbo/zpan/internal/app/repo"
	"github.com/saltbo/zpan/internal/app/usecase/storage"
	"github.com/saltbo/zpan/internal/pkg/auth"
	"github.com/samber/lo"
	"github.com/spf13/viper"

	"github.com/saltbo/zpan/internal/pkg/bind"
)

type StorageResource struct {
	jwtutil.JWTUtil

	storageRepo         repo.Storage
	storageUc           storage.Storage
	cloudStorageScanner *storage.CloudStorageScanner
}

func NewStorageResource(storageRepo repo.Storage, storageUc storage.Storage, cloudStorageScanner *storage.CloudStorageScanner) *StorageResource {
	return &StorageResource{storageRepo: storageRepo, storageUc: storageUc, cloudStorageScanner: cloudStorageScanner}
}

func (rs *StorageResource) Register(router *gin.RouterGroup) {
	router.GET("/storages/:id", rs.find)
	router.GET("/storages", rs.findAll)
	router.POST("/storages", rs.create)
	router.PUT("/storages/:id", rs.update)
	router.DELETE("/storages/:id", rs.delete)
	router.POST("/storages/:id/scan", rs.scanObjects)
}

func (rs *StorageResource) find(c *gin.Context) {
	ret, err := rs.storageRepo.Find(c, ginutil.ParamInt64(c, "id"))
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSONData(c, ret)

}

func (rs *StorageResource) findAll(c *gin.Context) {
	p := new(bind.StorageQuery)
	if err := c.Bind(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	list, total, err := rs.storageRepo.FindAll(c, &repo.StorageFindOptions{Limit: p.Limit, Offset: p.Offset})
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	lo.Map(list, func(item *entity.Storage, index int) *entity.Storage {
		ak_prefix := item.AccessKey[:5]
		ak_suffix := item.AccessKey[len(item.AccessKey)-3:]
		item.AccessKey = ak_prefix + strings.Repeat("*", len(item.AccessKey)-8) + ak_suffix
		item.SecretKey = strings.Repeat("*", len(item.SecretKey))
		return item
	})

	ginutil.JSONList(c, list, total)
}

func (rs *StorageResource) create(c *gin.Context) {
	p := new(bind.StorageBody)
	if err := c.Bind(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	if err := rs.storageUc.Create(c, p.Model()); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}

func (rs *StorageResource) update(c *gin.Context) {
	p := new(bind.StorageBody)
	if err := c.Bind(p); err != nil {
		ginutil.JSONBadRequest(c, err)
		return
	}

	if err := rs.storageRepo.Update(c, ginutil.ParamInt64(c, "id"), p.Model()); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}

func (rs *StorageResource) delete(c *gin.Context) {
	if err := rs.storageRepo.Delete(c, ginutil.ParamInt64(c, "id")); err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	ginutil.JSON(c)
}

// scanObjects godoc
// @Tags Storages
// @Summary Scan and import existing objects from storage
// @Description Scan cloud storage and import existing files as Matter records
// @Accept json
// @Produce json
// @Security OAuth2Application[admin]
// @Param id path int true "Storage ID"
// @Param body body bind.ScanStorageRequest false "Scan parameters"
// @Success 200 {object} bind.ScanStorageResponse
// @Failure 400 {object} httputil.JSONResponse
// @Failure 500 {object} httputil.JSONResponse
// @Router /storages/{id}/scan [post]
func (rs *StorageResource) scanObjects(c *gin.Context) {
	// Check if share.all_files is enabled
	if !viper.GetBool("share.all_files") {
		ginutil.JSONBadRequest(c, errors.New("Non-shared cloud storage. Scanning prohibited."))
		return
	}

	p := new(bind.CloudStorageScanRequest)
	if err := c.ShouldBindJSON(p); err != nil {
		// Allow empty body
		if c.Request.Body != nil {
			c.Request.Body.Close()
		}
	}

	storageID := ginutil.ParamInt64(c, "id")
	uid := auth.UidGet(c)

	// Call scanner
	result, err := rs.cloudStorageScanner.ScanObjects(c, uid, storageID, p.Prefix)
	if err != nil {
		ginutil.JSONServerError(c, err)
		return
	}

	// Convert to response
	resp := &bind.CloudStorageScanResponse{
		Total:   result.Total,
		Created: result.Created,
		Skipped: result.Skipped,
		Failed:  result.Failed,
		Errors:  result.Errors,
	}

	ginutil.JSONData(c, resp)
}
