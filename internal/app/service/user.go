package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/saltbo/gopkg/regexputil"
	"github.com/saltbo/gopkg/strutil"
	"github.com/saltbo/zpan/internal/app/entity"
	"github.com/saltbo/zpan/internal/pkg/ldap"
	"golang.org/x/crypto/bcrypt"

	"github.com/saltbo/zpan/internal/app/dao"
	"github.com/saltbo/zpan/internal/app/model"
)

// LDAPPasswordPrefix marks a user as LDAP-only, prevents local password auth
const LDAPPasswordPrefix = "!LDAP"

type User struct {
	dUser *dao.User
	dOpt  *dao.Option

	sToken *Token
	sMail  *Mail
	auth   *ldap.LDAPAuthenticator
}

func NewUser() *User {
	return &User{
		dUser: dao.NewUser(),
		dOpt:  dao.NewOption(),

		sToken: NewToken(),
		sMail:  NewMail(),
		auth:   ldap.Init(),
	}
}

func (u *User) Signup(email, password string, opt model.UserCreateOption) (*model.User, error) {
	if _, exist := u.dUser.TicketExist(opt.Ticket); !exist && opt.Ticket != "" {
		return nil, fmt.Errorf("invalid ticket")
	}

	// 创建基本信息
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Email:    email,
		Username: fmt.Sprintf("mu%s", strutil.RandomText(18)),
		Password: string(hashedPwd),
		Roles:    opt.Roles,
		Ticket:   strutil.RandomText(6),
	}
	mUser, err := u.dUser.Create(user, opt.StorageMax)
	if err != nil {
		return nil, err
	}

	// 如果如果启用了发信邮箱则发送一份激活邮件给用户
	if u.sMail.Enabled() {
		token, err := u.sToken.Create(mUser.IDString(), mUser.Username, 3600*24, mUser.Roles)
		if err != nil {
			return nil, err
		}

		return mUser, u.sMail.NotifyActive(opt.Origin, email, token)
	}

	return mUser, nil
}

func (u *User) Active(token string) error {
	rc, err := u.sToken.Verify(token)
	if err != nil {
		return err
	}

	uid, _ := strconv.ParseInt(rc.Subject, 10, 64)
	user, err := u.dUser.Find(uid)
	if err != nil {
		return err
	} else if user.Status >= model.StatusActivated {
		return fmt.Errorf("account already activated")
	}

	u.dUser.UpdateStorage(uid, entity.UserStorageActiveSize) // 激活即送1G空间
	return u.dUser.Activate(uid)
}

func (u *User) SignIn(usernameOrEmail, password string, ttl int) (*model.User, error) {
	var user *model.User
	var exist bool
	
	// Try LDAP authentication if enabled
	ldapEnabled := u.auth != nil && u.auth.IsEnabled()
	if ldapEnabled {
		email, ldapErr := u.auth.Authenticate(usernameOrEmail, password)
		if ldapErr == nil && email != "" {
			// LDAP authentication successful
			user, exist = u.dUser.EmailExist(email)
			if !exist {
				// Create new user from LDAP
				user = &model.User{
					Email:    email,
					Username: fmt.Sprintf("mu%s", strutil.RandomText(18)),
					Password: LDAPPasswordPrefix + strutil.RandomText(32),
					Roles:    model.RoleMember,
					Ticket:   strutil.RandomText(6),
					Status:   model.StatusActivated,
				}
				mUser, err := u.dUser.Create(user, 0)
				if err != nil {
					return nil, fmt.Errorf("failed to create user: %w", err)
				}
				user = mUser
			}
		} else {
			// LDAP failed - only allow admin fallback to local password
			user, exist = u.findUserByUsernameOrEmail(usernameOrEmail)
			if !exist || !u.isAdmin(user) {
				return nil, fmt.Errorf("authentication failed")
			}
			// Verify local password for admin fallback
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
				return nil, fmt.Errorf("invalid password")
			}
		}
	} else {
		// LDAP disabled - use local authentication only
		userFinder := u.dUser.UsernameExist
		if regexputil.EmailRegex.MatchString(usernameOrEmail) {
			userFinder = u.dUser.EmailExist
		}
		user, exist = userFinder(usernameOrEmail)
		if !exist {
			return nil, fmt.Errorf("user not exist")
		}
		// Verify local password
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			return nil, fmt.Errorf("invalid password")
		}
	}

	// Check if account is activated
	if u.sMail.Enabled() && !user.Activated() {
		return nil, fmt.Errorf("account is not activated")
	}

	token, err := u.sToken.Create(user.IDString(), user.Username, ttl, user.Roles)
	if err != nil {
		return nil, err
	}
	user.Token = token
	return user, nil
}

func (u *User) SignOut() {

}

// findUserByUsernameOrEmail finds user by username or email
func (u *User) findUserByUsernameOrEmail(usernameOrEmail string) (*model.User, bool) {
	if regexputil.EmailRegex.MatchString(usernameOrEmail) {
		return u.dUser.EmailExist(usernameOrEmail)
	}
	return u.dUser.UsernameExist(usernameOrEmail)
}

// isAdmin checks if user has admin role
func (u *User) isAdmin(user *model.User) bool {
	if user == nil {
		return false
	}
	roles := strings.Split(user.Roles, ",")
	for _, role := range roles {
		if strings.TrimSpace(role) == model.RoleAdmin {
			return true
		}
	}
	return false
}

func (u *User) PasswordUpdate(uid int64, oldPwd, newPwd string) error {
	user, err := u.dUser.Find(uid)
	if err != nil {
		return err
	} else if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPwd)); err != nil {
		return fmt.Errorf("error password")
	} else if user.Username == "demo" {
		return fmt.Errorf("user demo not allowed change password")
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPwd)
	return u.dUser.Update(user)
}

func (u *User) PasswordResetApply(origin, email string) error {
	user, ok := u.dUser.EmailExist(email)
	if !ok {
		return fmt.Errorf("email not exist")
	}

	// issue a short-term token for password reset
	token, err := u.sToken.Create(user.IDString(), user.Username, 300)
	if err != nil {
		return err
	}

	return u.sMail.NotifyPasswordReset(origin, email, token)
}

func (u *User) PasswordReset(token, password string) error {
	rc, err := u.sToken.Verify(token)
	if err != nil {
		return err
	}

	return u.dUser.PasswordReset(rc.Uid(), password)
}

func (u *User) InviteRequired() bool {
	opts, err := u.dOpt.Get(model.OptSite)
	if err != nil {
		return false
	}

	return opts.GetBool("invite_required")
}
