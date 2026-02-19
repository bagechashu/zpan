package dao

import (
	"errors"
	"fmt"
	"strings"

	"github.com/saltbo/zpan/internal/app/entity"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/saltbo/zpan/internal/app/model"
)

type User struct {
}

func NewUser() *User {
	return &User{}
}

func (u *User) Find(uid int64) (*model.User, error) {
	user := new(model.User)
	if err := gdb.First(user, uid).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("user not exist")
	}

	gdb.Model(user).Association("Profile").Find(&user.Profile)
	gdb.Model(user).Association("Storage").Find(&user.Storage)

	return user.Format(), nil
}

func (u *User) FindByUsername(username string) (*model.User, error) {
	user := new(model.User)
	if err := gdb.First(user, "username=?", username).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("user not exist")
	} else if err != nil {
		return nil, err
	}

	return user.Format(), nil
}

func (u *User) FindAll(query *Query) (list []*model.User, total int64, err error) {
	sn := gdb.Model(&model.User{})
	if len(query.Params) > 0 {
		sn = sn.Where(query.SQL(), query.Params...)
	}
	sn.Count(&total)
	err = sn.Offset(query.Offset).Limit(query.Limit).Preload(clause.Associations).Find(&list).Error
	for _, user := range list {
		user = user.Format()
	}
	return
}

func (u *User) EmailExist(email string) (*model.User, bool) {
	return u.userExist("email", email)
}

func (u *User) UsernameExist(username string) (*model.User, bool) {
	return u.userExist("username", username)
}

func (u *User) TicketExist(ticket string) (*model.User, bool) {
	return u.userExist("ticket", ticket)
}

func (u *User) userExist(k, v string) (*model.User, bool) {
	user := new(model.User)
	err := gdb.Where(k+"=?", v).First(user).Error
	if err != nil {
		// if errors.Is(err, gorm.ErrRecordNotFound) {
		// 	// User does not exist
		// 	return nil, false
		// }
		// Other database errors: log and return not found to prevent using invalid data
		return nil, false
	}

	return user.Format(), true
}

func (u *User) Create(user *model.User, storageMax uint64) (*model.User, error) {
	if _, exist := u.EmailExist(user.Email); exist {
		return nil, fmt.Errorf("email already exist")
	}

	// Use transaction to ensure atomicity of user, profile, and storage creation
	err := gdb.Transaction(func(tx *gorm.DB) error {
		// Create user record (database will assign user.Id)
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		// Create profile record with assigned user.Id
		profile := &model.UserProfile{
			Uid:      user.Id,
			Nickname: user.Email[:strings.Index(user.Email, "@")],
		}
		if err := tx.Create(profile).Error; err != nil {
			return fmt.Errorf("failed to create user profile: %w", err)
		}

		// Create storage record
		storage := entity.UserStorage{
			Uid: user.Id,
			Max: entity.UserStorageDefaultSize,
		}
		if storageMax > 0 {
			storage.Max = storageMax
		}
		if err := tx.Create(&storage).Error; err != nil {
			return fmt.Errorf("failed to create user storage: %w", err)
		}

		// Set associated data without reloading from database
		user.Profile = *profile
		user.Storage = storage
		return nil
	})
	return user, err
}

func (u *User) Activate(uid int64) error {
	user, err := u.Find(uid)
	if err != nil {
		return err
	}

	if err := gdb.Model(user).Update("status", model.StatusActivated).Error; err != nil {
		return err
	}

	return nil
}

// PasswordReset update the new password
func (u *User) PasswordReset(uid int64, newPwd string) error {
	user, err := u.Find(uid)
	if err != nil {
		return err
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := gdb.Model(user).Update("password", string(hashedPwd)).Error; err != nil {
		return err
	}
	// record the old password

	return nil
}

func (u *User) Update(user *model.User) error {
	return gdb.Save(user).Error
}

func (u *User) UpdateStatus(uid int64, status uint8) error {
	return gdb.Model(model.User{}).Where("id=?", uid).Update("status", status).Error
}

func (u *User) Delete(user *model.User) error {
	return gdb.Delete(user).Error
}

func (u *User) UpdateProfile(uid int64, up *model.UserProfile) error {
	// 使用 Upsert: 如果 uid 存在则更新，不存在则插入
	up.Uid = uid
	return gdb.Clauses(clause.OnConflict{
		// uid 有唯一约束，冲突时更新所有列
		UpdateAll: true,
	}).Create(up).Error
}

func (u *User) UpdateStorage(uid int64, quota uint64) error {
	return gdb.Model(&entity.UserStorage{}).Where("uid=?", uid).Update("max", quota).Error
}
