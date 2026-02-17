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

	// 验证 LDAP 配置
	if config.Enabled {
		if config.Url == "" {
			panic(`CRITICAL: LDAP is enabled but ldap.url is not configured!`)
		}
		if config.BindDN == "" {
			panic(`CRITICAL: LDAP is enabled but ldap.bind_dn is not configured!`)
		}
		if config.BindPassword == "" || config.BindPassword == "${LDAP_PASSWORD}" {
			panic(`CRITICAL: LDAP is enabled but bind_password is not configured! 
Please set the LDAP_PASSWORD environment variable.
Example:
  export LDAP_PASSWORD="your-ldap-password"
  
Or in docker-compose/systemd:
  environment:
    - LDAP_PASSWORD=your-ldap-password`)
		}
	}

	return NewLDAPAuthenticator(config)
}
