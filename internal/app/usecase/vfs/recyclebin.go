package vfs

import (
	"context"
	"strings"

	"github.com/saltbo/zpan/internal/app/repo"
	"github.com/saltbo/zpan/internal/app/usecase/storage"
	"github.com/saltbo/zpan/internal/pkg/logger"
)

var _ RecycleBinFs = (*RecycleBin)(nil)

type RecycleBin struct {
	recycleRepo repo.RecycleBin
	matterRepo  repo.Matter
	userRepo    repo.User
	storage     storage.Storage
}

func NewRecycleBin(recycleRepo repo.RecycleBin, matterRepo repo.Matter, userRepo repo.User, storage storage.Storage) *RecycleBin {
	return &RecycleBin{recycleRepo: recycleRepo, matterRepo: matterRepo, userRepo: userRepo, storage: storage}
}

func (rb *RecycleBin) Recovery(ctx context.Context, alias string) error {
	rbv, err := rb.recycleRepo.Find(ctx, alias)
	if err != nil {
		return err
	}

	if err := rb.matterRepo.Recovery(ctx, rbv.Mid); err != nil {
		return err
	}

	return rb.recycleRepo.Delete(ctx, alias)
}

func (rb *RecycleBin) Delete(ctx context.Context, alias string) error {
	m, err := rb.recycleRepo.Find(ctx, alias)
	if err != nil {
		return err
	}

	matter, err := rb.matterRepo.FindWith(ctx, &repo.MatterFindWithOption{Id: m.Mid, Deleted: true})
	if err != nil {
		return err
	}

	provider, err := rb.storage.GetProvider(ctx, matter.Sid)
	if err != nil {
		return err
	}

	objects, _ := rb.matterRepo.GetObjects(ctx, matter.Id)
	logger.Debug("recyclebin delete obj: %v", objects)
	
	// For directories, add the directory object itself (path + "/" in storage)
	// This ensures the directory marker object in OBS is also deleted
	if matter.IsDir() {
		storage, err := rb.storage.Get(ctx, matter.Sid)
		if err == nil && storage != nil {
			// Build the directory object key: rootPath + matter.FullPath()
			dirKey := storage.RootPath + strings.TrimLeft(matter.FullPath(), "/")
			objects = append(objects, dirKey)
		}
	}
	logger.Debug("recyclebin delete obj: %v", objects)
	
	if len(objects) != 0 {
		if err := provider.ObjectsDelete(objects); err != nil {
			return err
		}
	}

	defer rb.userRepo.UserStorageUsedDecr(ctx, matter)
	return rb.recycleRepo.Delete(ctx, alias)
}

func (rb *RecycleBin) Clean(ctx context.Context, sid, uid int64) error {
	rbs, _, err := rb.recycleRepo.FindAll(ctx, &repo.RecycleBinFindOptions{Sid: sid, Uid: uid})
	if err != nil {
		return err
	}

	for _, rbMatter := range rbs {
		if err := rb.Delete(ctx, rbMatter.Alias); err != nil {
			return err
		}
	}

	return nil
}
