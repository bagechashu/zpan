package uploader

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/saltbo/zpan/internal/app/entity"
	"github.com/saltbo/zpan/internal/app/repo"
	"github.com/saltbo/zpan/internal/app/usecase/storage"
)

var _ Uploader = (*CloudUploader)(nil)

const uploadDoneTimeout = 30 * time.Minute

type CloudUploader struct {
	storage    storage.Storage
	matterRepo repo.Matter
}

func NewCloudUploader(storage storage.Storage, matterRepo repo.Matter) *CloudUploader {
	return &CloudUploader{storage: storage, matterRepo: matterRepo}
}

func (u *CloudUploader) CreateUploadURL(ctx context.Context, m *entity.Matter) error {
	provider, err := u.storage.GetProvider(ctx, m.Sid)
	if err != nil {
		return err
	}

	s, err := u.storage.Get(ctx, m.Sid)
	if err != nil {
		return err
	}

	m.BuildObject(s.RootPath, s.FilePath)
	urlStr, header, err := provider.SignedPutURL(m.Object, m.Type, m.Size, s.PublicRead())
	if err != nil {
		return err
	}

	m.Uploader["upURL"] = urlStr
	m.Uploader["upHeaders"] = header
	return nil
}

func (u *CloudUploader) CreateVisitURL(ctx context.Context, m *entity.Matter) error {
	provider, err := u.storage.GetProvider(ctx, m.Sid)
	if err != nil {
		return err
	}

	s, err := u.storage.Get(ctx, m.Sid)
	if err != nil {
		return err
	}

	if s.PublicRead() {
		m.URL = provider.PublicURL(m.Object)
		return nil
	}

	link, err := provider.SignedGetURL(m.Object, m.Name)
	m.URL = link
	return err
}

func (u *CloudUploader) UploadDone(ctx context.Context, m *entity.Matter) error {
	if !m.CreatedAt.IsZero() && time.Since(m.CreatedAt) >= uploadDoneTimeout {
		if err := u.matterRepo.Delete(ctx, m.Id); err != nil {
			return err
		}

		return fmt.Errorf("upload timeout, matter deleted")
	}

	provider, err := u.storage.GetProvider(ctx, m.Sid)
	if err != nil {
		return err
	}

	if _, err := provider.Head(m.Object); err != nil {
		return err
	}

	m.SetUploadedAt()
	return u.matterRepo.Update(ctx, m.Id, m)
}

func (u *CloudUploader) MoveObject(ctx context.Context, m *entity.Matter, to string) (string, error) {
	if m.IsDir() || m.Object == "" {
		return m.Object, nil
	}

	s, err := u.storage.Get(ctx, m.Sid)
	if err != nil {
		return "", err
	}

	provider, err := u.storage.GetProvider(ctx, m.Sid)
	if err != nil {
		return "", err
	}

	newObject := path.Join(s.RootPath, strings.Trim(to, "/"), m.Name)
	if newObject == m.Object {
		return m.Object, nil
	}

	if err := provider.Move(m.Object, newObject); err != nil {
		return "", err
	}

	return newObject, nil
}
