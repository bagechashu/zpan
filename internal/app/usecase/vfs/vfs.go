package vfs

import (
	"context"
	"fmt"
	"path"

	"github.com/robfig/cron"
	"github.com/saltbo/zpan/internal/app/entity"
	"github.com/saltbo/zpan/internal/app/repo"
	"github.com/saltbo/zpan/internal/app/usecase/uploader"
)

var _ VirtualFs = (*Vfs)(nil)

type Vfs struct {
	matterRepo     repo.Matter
	recycleBinRepo repo.RecycleBin
	userRepo       repo.User
	uploader       uploader.Uploader
	eventWorker    *EventWorker
}

type moveObjectCtxkeyType string

const moveObjectCtx moveObjectCtxkeyType = "vfs.move.object.enabled"

func CtxSetShareAllFilesStatus(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, moveObjectCtx, enabled)
}

func shouldMoveObject(ctx context.Context) bool {
	v, ok := ctx.Value(moveObjectCtx).(bool)
	if !ok {
		return false
	}

	return v
}

func NewVfs(matterRepo repo.Matter, recycleBinRepo repo.RecycleBin, userRepo repo.User, uploader uploader.Uploader) *Vfs {
	vfs := &Vfs{matterRepo: matterRepo, recycleBinRepo: recycleBinRepo, userRepo: userRepo, uploader: uploader, eventWorker: NewWorker()}
	vfs.eventWorker.registerEventHandler(EventActionCreated, vfs.matterCreatedEventHandler)
	vfs.eventWorker.registerEventHandler(EventActionDeleted, vfs.matterDeletedEventHandler)
	_ = cron.New().AddFunc("30 1 * * *", vfs.cleanExpiredMatters)
	go vfs.eventWorker.Run()
	return vfs
}

func (v *Vfs) Create(ctx context.Context, m *entity.Matter) error {
	if !m.IsDir() {
		us, err := v.userRepo.GetUserStorage(ctx, m.Uid)
		if err != nil {
			return fmt.Errorf("error getting user storage: %v", err)
		} else if us.Overflowed(m.Size) {
			return fmt.Errorf("insufficient storage space")
		}

		if err := v.uploader.CreateUploadURL(ctx, m); err != nil {
			return fmt.Errorf("failed to create upload URL: %v", err)
		}
	}

	// Try to create the matter in database
	if err := v.matterRepo.Create(ctx, m); err != nil {
		// Log the error for debugging
		return fmt.Errorf("failed to create matter record: %v", err)
	}

	// Send event only after successful database creation
	// This prevents background tasks from processing records that don't exist in the database
	if !m.IsDir() {
		defer v.eventWorker.sendEvent(EventActionCreated, m)
	}

	return nil
}

func (v *Vfs) List(ctx context.Context, option *repo.MatterListOption) ([]*entity.Matter, int64, error) {
	return v.matterRepo.FindAll(ctx, option)
}

func (v *Vfs) Get(ctx context.Context, alias string) (*entity.Matter, error) {
	matter, err := v.matterRepo.FindByAlias(ctx, alias)
	if err != nil {
		return nil, err
	}

	if matter.IsDir() {
		return matter, nil
	}

	return matter, v.uploader.CreateVisitURL(ctx, matter)
}

func (v *Vfs) Rename(ctx context.Context, alias string, newName string) error {
	m, err := v.matterRepo.FindByAlias(ctx, alias)
	if err != nil {
		return err
	}

	if exist := v.matterRepo.PathExistWithScope(ctx, path.Join(m.Parent, newName), m.Uid, m.Sid, m.Id); exist {
		return fmt.Errorf("dir already has the same name file")
	}

	oldName := m.Name
	m.Name = newName

	if shouldMoveObject(ctx) && !m.IsDir() && m.Object != "" {
		if v.uploader == nil {
			m.Name = oldName
			return fmt.Errorf("uploader is required to rename file object")
		}

		newObject, err := v.uploader.MoveObject(ctx, m, m.Parent)
		if err != nil {
			m.Name = oldName
			return err
		}
		m.Object = newObject
	}

	return v.matterRepo.Update(ctx, m.Id, m)
}

func (v *Vfs) Move(ctx context.Context, alias string, to string) error {
	m, err := v.matterRepo.FindByAlias(ctx, alias)
	if err != nil {
		return err
	}

	if exist := v.matterRepo.PathExistWithScope(ctx, path.Join(to, m.Name), m.Uid, m.Sid, m.Id); exist {
		return fmt.Errorf("dir already has the same name file")
	}

	if shouldMoveObject(ctx) && !m.IsDir() && m.Object != "" {
		if v.uploader == nil {
			return fmt.Errorf("uploader is required to move file object")
		}

		newObject, err := v.uploader.MoveObject(ctx, m, to)
		if err != nil {
			return err
		}
		m.Object = newObject
	}

	m.Parent = to
	return v.matterRepo.Update(ctx, m.Id, m)
}

func (v *Vfs) Copy(ctx context.Context, alias string, to string) (*entity.Matter, error) {
	m, err := v.matterRepo.FindByAlias(ctx, alias)
	if err != nil {
		return nil, err
	}

	return v.matterRepo.Copy(ctx, m.Id, to)
}

func (v *Vfs) Delete(ctx context.Context, alias string) error {
	m, err := v.matterRepo.FindByAlias(ctx, alias)
	if err != nil {
		return err
	}

	if err := v.matterRepo.Delete(ctx, m.Id); err != nil {
		return err
	}

	defer v.eventWorker.sendEvent(EventActionDeleted, m)
	rb := m.BuildRecycleBinItem()
	return v.recycleBinRepo.Create(ctx, rb)
}
