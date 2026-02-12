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
}

// Init 从 viper 配置中初始化 LDAP 配置并返回认证器
func Init() *LDAPAuthenticator {
	config := &LDAPConfig{
		Enabled:      viper.GetBool("ldap.enabled"),
		Url:          viper.GetString("ldap.url"),
		BindDN:       viper.GetString("ldap.bind_dn"),
		BindPassword: viper.GetString("ldap.bind_password"),
		BaseDN:       viper.GetString("ldap.base_dn"),
		UserFilter:   viper.GetString("ldap.user_filter"),
		UserAttr:     viper.GetString("ldap.user_attribute"),
		StartTLS:     viper.GetBool("ldap.start_tls"),
	}

	return NewLDAPAuthenticator(config)
}
