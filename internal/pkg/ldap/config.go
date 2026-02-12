package ldap

import "github.com/spf13/viper"

type LDAPConfig struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	Url          string `json:"url" yaml:"url"`
	BindDN       string `json:"bind_dn" yaml:"bind_dn"`
	BindPassword string `json:"bind_password" yaml:"bind_password"`
	BaseDN       string `json:"base_dn" yaml:"base_dn"`
	UserFilter   string `json:"user_filter" yaml:"user_filter"`
	UserAttr     string `json:"user_attribute" yaml:"user_attribute"`
	StartTLS     bool   `json:"start_tls" yaml:"start_tls"` // Use STARTTLS
	Timeout      int    `json:"timeout" yaml:"timeout"`      // Connection timeout in seconds (default: 2)
}

// Init 从 viper 配置中初始化 LDAP 配置并返回认证器
func Init() *LDAPAuthenticator {
	timeout := viper.GetInt("ldap.timeout")
	if timeout <= 0 {
		timeout = 2 // Default to 2 seconds
	}

	config := &LDAPConfig{
		Enabled:      viper.GetBool("ldap.enabled"),
		Url:          viper.GetString("ldap.url"),
		BindDN:       viper.GetString("ldap.bind_dn"),
		BindPassword: viper.GetString("ldap.bind_password"),
		BaseDN:       viper.GetString("ldap.base_dn"),
		UserFilter:   viper.GetString("ldap.user_filter"),
		UserAttr:     viper.GetString("ldap.user_attribute"),
		StartTLS:     viper.GetBool("ldap.start_tls"),
		Timeout:      timeout,
	}

	return NewLDAPAuthenticator(config)
}
