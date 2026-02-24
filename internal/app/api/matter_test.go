package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/saltbo/zpan/internal/app/entity"
	"github.com/saltbo/zpan/internal/app/usecase/uploader"
	"github.com/saltbo/zpan/internal/app/usecase/vfs"
	"github.com/saltbo/zpan/internal/mock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func setupMatterTestRouter(t *testing.T, matter *entity.Matter, moveObjectFn func(ctx context.Context, m *entity.Matter, to string) (string, error)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	matterRepo := mock.NewMatter()
	assert.NoError(t, matterRepo.Create(context.Background(), matter))

	up := &uploader.FakeUploader{MoveObjectFn: moveObjectFn}
	fs := vfs.NewVfs(matterRepo, nil, nil, up)

	r := gin.New()
	group := r.Group("/api")
	NewFileResource(fs, up).Register(group)
	return r
}

func TestPatchMatterName_ShareAllFilesBehavior(t *testing.T) {
	tests := []struct {
		name             string
		shareAllFiles    bool
		expectMoveObject bool
		expectObject     string
	}{
		{
			name:             "share_all_files=false only updates db name",
			shareAllFiles:    false,
			expectMoveObject: false,
			expectObject:     "bucket/docs/original.txt",
		},
		{
			name:             "share_all_files=true syncs object rename",
			shareAllFiles:    true,
			expectMoveObject: true,
			expectObject:     "bucket/docs/new.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("share.all_files", tt.shareAllFiles)

			matter := &entity.Matter{
				Id:     1,
				Alias:  "alias-name",
				Uid:    100,
				Sid:    1,
				Parent: "docs",
				Name:   "original.txt",
				Object: "bucket/docs/original.txt",
			}

			moveObjectCalled := 0
			r := setupMatterTestRouter(t, matter, func(ctx context.Context, m *entity.Matter, to string) (string, error) {
				moveObjectCalled++
				return "bucket/docs/" + m.Name, nil
			})

			req := httptest.NewRequest(http.MethodPatch, "/api/matters/alias-name/name", strings.NewReader(`{"name":"new.txt"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "new.txt", matter.Name)
			assert.Equal(t, tt.expectObject, matter.Object)
			if tt.expectMoveObject {
				assert.Equal(t, 1, moveObjectCalled)
			} else {
				assert.Equal(t, 0, moveObjectCalled)
			}
		})
	}
}

func TestPatchMatterLocation_ShareAllFilesBehavior(t *testing.T) {
	tests := []struct {
		name             string
		shareAllFiles    bool
		expectMoveObject bool
		expectObject     string
	}{
		{
			name:             "share_all_files=false only updates db parent",
			shareAllFiles:    false,
			expectMoveObject: false,
			expectObject:     "bucket/docs/original.txt",
		},
		{
			name:             "share_all_files=true syncs object move",
			shareAllFiles:    true,
			expectMoveObject: true,
			expectObject:     "bucket/archive/original.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("share.all_files", tt.shareAllFiles)

			matter := &entity.Matter{
				Id:     2,
				Alias:  "alias-move",
				Uid:    100,
				Sid:    1,
				Parent: "docs",
				Name:   "original.txt",
				Object: "bucket/docs/original.txt",
			}

			moveObjectCalled := 0
			r := setupMatterTestRouter(t, matter, func(ctx context.Context, m *entity.Matter, to string) (string, error) {
				moveObjectCalled++
				return "bucket/" + strings.Trim(to, "/") + "/" + m.Name, nil
			})

			req := httptest.NewRequest(http.MethodPatch, "/api/matters/alias-move/location", strings.NewReader(`{"dir":"archive"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "archive", matter.Parent)
			assert.Equal(t, tt.expectObject, matter.Object)
			if tt.expectMoveObject {
				assert.Equal(t, 1, moveObjectCalled)
			} else {
				assert.Equal(t, 0, moveObjectCalled)
			}
		})
	}
}
