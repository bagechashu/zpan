package uploader

import (
	"context"

	"github.com/saltbo/zpan/internal/app/entity"
)

type FakeUploader struct {
	CreateUploadURLFn func(ctx context.Context, m *entity.Matter) error
	MoveObjectFn      func(ctx context.Context, m *entity.Matter, to string) (string, error)
}

func (f *FakeUploader) CreateUploadURL(ctx context.Context, m *entity.Matter) error {
	return f.CreateUploadURLFn(ctx, m)
}

func (f *FakeUploader) CreateVisitURL(ctx context.Context, m *entity.Matter) error {
	return nil
}

func (f *FakeUploader) UploadDone(ctx context.Context, m *entity.Matter) error {
	return nil
}

func (f *FakeUploader) MoveObject(ctx context.Context, m *entity.Matter, to string) (string, error) {
	if f.MoveObjectFn != nil {
		return f.MoveObjectFn(ctx, m, to)
	}

	return m.Object, nil
}
