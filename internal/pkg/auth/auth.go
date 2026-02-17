package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/saltbo/zpan/internal/app/model"
)

const (
	ctxUidKey = "ctx-uid"

	cookieTokenKey = "z-token"
)

func UidSet(c *gin.Context, uid int64) {
	c.Set(ctxUidKey, uid)
}

func UidGet(c *gin.Context) int64 {
	return c.GetInt64(ctxUidKey)
}

func RoleSet(c *gin.Context, roles []string) {
	c.Set("role", roles)
}

func IsAdmin(c *gin.Context) bool {
	for _, s := range c.GetStringSlice("role") {
		if s == model.RoleAdmin {
			return true
		}
	}

	return false
}

// TokenCookieSet 安全地设置认证令牌 Cookie
// ✅ 已启用: HttpOnly（防止 XSS）、Secure（仅 HTTPS）、SameSite=Strict（防止 CSRF）
func TokenCookieSet(c *gin.Context, token string, expireSec int) {
	setSecureCookie(c, cookieTokenKey, token, expireSec)
}

// TokenCookieGet 读取认证令牌 Cookie
func TokenCookieGet(c *gin.Context) string {
	token, _ := c.Cookie(cookieTokenKey)
	return token
}

// setSecureCookie 设置安全的 Cookie，包含所有安全属性
// 参数:
//   - name: cookie 名称
//   - value: cookie 值（为空时清除 cookie）
//   - maxAge: cookie 过期时间（秒），0 表示立即过期，-1 表示删除
//
// 安全属性:
//   - HttpOnly: 防止 JavaScript 访问（防止 XSS 攻击）
//   - Secure: 仅在 HTTPS 连接上发送（防止中间人攻击，可通过配置禁用以支持开发环境）
//   - SameSite=Strict: 防止跨站请求伪造（CSRF）
//
// 参考: https://owasp.org/www-community/attacks/csrf
func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
	// Cookie Secure 属性开关，默认 true（生产级别）
	// 设置为 false 可在开发/测试环境使用 HTTP
	cookieSecure := viper.GetBool("jwt.cookie_secure")
	if !viper.IsSet("jwt.cookie_secure") {
		// 如果未配置，默认为 true（安全优先）
		cookieSecure = true
	}

	cookie := http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     "/",
		Domain:   "",
		Secure:   cookieSecure,            // 根据配置决定
		HttpOnly: true,                    // 防止 JavaScript 访问
		SameSite: http.SameSiteStrictMode, // 严格防止 CSRF
	}
	http.SetCookie(c.Writer, &cookie)
}

// ShareCookieSet 安全地设置分享访问 Cookie
// ✅ 已启用: HttpOnly（防止 XSS）、Secure（可配置）、SameSite=Strict（防止 CSRF）
// 说明: Secure 属性根据 jwt.cookie_secure 配置决定，便于开发/测试环境使用 HTTP
func ShareCookieSet(c *gin.Context, name, token string, expireSec int) {
	// Cookie Secure 属性开关，默认 true（生产级别）
	// 设置为 false 可在开发/测试环境使用 HTTP
	cookieSecure := viper.GetBool("jwt.cookie_secure")
	if !viper.IsSet("jwt.cookie_secure") {
		// 如果未配置，默认为 true（安全优先）
		cookieSecure = true
	}

	cookie := http.Cookie{
		Name:     name,
		Value:    token,
		MaxAge:   expireSec,
		Path:     "/",
		Domain:   "",
		Secure:   cookieSecure,            // 根据配置决定
		HttpOnly: true,                    // 防止 JavaScript 访问
		SameSite: http.SameSiteStrictMode, // 严格防止 CSRF
	}
	http.SetCookie(c.Writer, &cookie)
}
