package repo

import (
	"context"

	"github.com/saltbo/zpan/internal/app/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type User interface {
	GetUserStorage(ctx context.Context, uid int64) (*entity.UserStorage, error)
	UserStorageUsedIncr(ctx context.Context, matter *entity.Matter) error
	UserStorageUsedDecr(ctx context.Context, matter *entity.Matter) error
}

var _ User = (*UserDBQuery)(nil)

type UserDBQuery struct {
	DBQuery
}

func NewUserDBQuery(q DBQuery) *UserDBQuery {
	return &UserDBQuery{DBQuery: q}
}

func (u *UserDBQuery) GetUserStorage(ctx context.Context, uid int64) (*entity.UserStorage, error) {
	us, err := u.Q().UserStorage.WithContext(ctx).Where(u.Q().UserStorage.Uid.Eq(uid)).First()
	if err == nil {
		return us, nil
	}

	// If record not found, create a default storage quota
	if err == gorm.ErrRecordNotFound {
		us := &entity.UserStorage{
			Uid: uid,
			Max: entity.UserStorageDefaultSize,
		}
		if err := u.Q().UserStorage.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(us); err != nil {
			return nil, err
		}
		return us, nil
	}

	return nil, err
}

func (u *UserDBQuery) UserStorageUsedIncr(ctx context.Context, matter *entity.Matter) error {
	q := u.Q().UserStorage.WithContext(ctx).Where(u.Q().UserStorage.Uid.Eq(matter.Uid))
	_, err := q.Update(u.Q().UserStorage.Used, gorm.Expr("used+?", matter.Size))
	return err
}

func (u *UserDBQuery) UserStorageUsedDecr(ctx context.Context, matter *entity.Matter) error {
	q := u.Q().UserStorage.WithContext(ctx).Where(u.Q().UserStorage.Uid.Eq(matter.Uid))
	userStorage, err := q.First()
	if err != nil {
		return err
	}

	used := uint64(matter.Size)
	if used > userStorage.Used {
		used = userStorage.Used // 使用量不能变成负数
	}

	_, err = q.Update(u.Q().UserStorage.Used, gorm.Expr("used-?", used))
	return err
}
