package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/saltbo/gopkg/jwtutil"
	"github.com/spf13/viper"
)

var jwtUtilInitialized = false

type Token struct {
}

// NewToken 创建 Token 实例，确保 JWT 密钥仅初始化一次
func NewToken() *Token {
	if !jwtUtilInitialized {
		secret := viper.GetString("jwt.secret")
		if secret == "" || secret == "change-me-to-a-random-secret-key-at-least-32-chars" || secret == "${JWT_SECRET}" {
			panic(`CRITICAL: JWT secret not configured! Please set the JWT_SECRET environment variable.
Example:
  export JWT_SECRET=$(openssl rand -base64 32)
  
Or in docker-compose/systemd:
  environment:
    - JWT_SECRET=your-generated-secret-here
  
The secret must be at least 32 characters long.`)
		}
		jwtutil.Init(secret)
		jwtUtilInitialized = true

		// 注意: config 中的 jwt.refresh_token_ttl 当前未使用
		// 将来实现 OAuth2.0 Refresh Token 流程时会使用此配置
	}
	return &Token{}
}

// Create 创建一个新的 JWT token
// uid: 用户 ID
// username: 用户名
// ttl: token 的过期时间（秒）
// roles: 用户角色列表
func (s *Token) Create(uid string, username string, ttl int, roles ...string) (string, error) {
	return jwtutil.Issue(NewRoleClaims(uid, username, ttl, roles))
}

// Verify 验证 JWT token 的有效性
func (s *Token) Verify(tokenStr string) (*RoleClaims, error) {
	token, err := jwtutil.Verify(tokenStr, &RoleClaims{})
	if err != nil {
		return nil, fmt.Errorf("token valid failed: %s", err)
	}

	return token.Claims.(*RoleClaims), nil
}

type RoleClaims struct {
	jwt.StandardClaims

	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

func NewRoleClaims(uid string, username string, ttl int, roles []string) *RoleClaims {
	timeNow := time.Now()
	return &RoleClaims{
		StandardClaims: jwt.StandardClaims{
			Issuer:    "zpan",
			Audience:  "zpanUsers",
			ExpiresAt: timeNow.Add(time.Duration(ttl) * time.Second).Unix(),
			IssuedAt:  timeNow.Unix(),
			NotBefore: timeNow.Unix(),
			Subject:   uid,
		},
		Username: username,
		Roles:    roles,
	}
}

func (rc *RoleClaims) Uid() int64 {
	uid, _ := strconv.ParseInt(rc.Subject, 10, 64)
	return uid
}
