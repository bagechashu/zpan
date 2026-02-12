package ldap

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
)

type LDAPAuthenticator struct {
	config *LDAPConfig
}

// NewLDAPAuthenticator creates a new LDAP authenticator with given config
func NewLDAPAuthenticator(config *LDAPConfig) *LDAPAuthenticator {
	return &LDAPAuthenticator{
		config: config,
	}
}

// IsEnabled returns whether LDAP authentication is enabled
func (la *LDAPAuthenticator) IsEnabled() bool {
	return la.config.Enabled
}

// Authenticate verifies the username and password against LDAP server
// Returns user email/username if successful, error otherwise
func (la *LDAPAuthenticator) Authenticate(username, password string) (string, error) {
	if !la.config.Enabled {
		return "", fmt.Errorf("LDAP is not enabled")
	}

	if username == "" || password == "" {
		return "", fmt.Errorf("username or password cannot be empty")
	}

	// Establish connection to LDAP server
	conn, err := la.dial()
	if err != nil {
		return "", fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer conn.Close()

	// Bind with admin credentials to search for user
	if err := conn.Bind(la.config.BindDN, la.config.BindPassword); err != nil {
		return "", fmt.Errorf("failed to bind as admin: %w", err)
	}

	// Search for the user
	userDN, email, err := la.searchUser(conn, username)
	if err != nil {
		return "", err
	}

	// Try to bind as the user with provided password
	userConn, err := la.dial()
	if err != nil {
		return "", fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer userConn.Close()

	if err := userConn.Bind(userDN, password); err != nil {
		return "", fmt.Errorf("invalid username or password")
	}

	// If email is empty, use username as fallback
	if email == "" {
		email = username
	}

	return email, nil
}

// dial creates a new connection to LDAP server
func (la *LDAPAuthenticator) dial() (*ldap.Conn, error) {
	// No verify server certificate for simplicity.
	tlsConfig := &tls.Config{InsecureSkipVerify: true}

	schema := strings.Split(la.config.Url, "://")[0]
	var dialOpts []ldap.DialOpt
	timeoutDuration := time.Duration(la.config.Timeout) * time.Second
	dialOpts = append(dialOpts, ldap.DialWithDialer(&net.Dialer{Timeout: timeoutDuration}))

	// For LDAPS, pass TLS config during dial
	if schema == "ldaps" {
		dialOpts = append(dialOpts, ldap.DialWithTLSConfig(tlsConfig))
	}

	conn, err := ldap.DialURL(la.config.Url, dialOpts...)
	if err != nil {
		return nil, err
	}

	// If using StartTLS, upgrade the connection after binding
	if schema == "ldap" && la.config.StartTLS {
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Set operation timeout (for search and bind operations)
	conn.SetTimeout(time.Duration(la.config.Timeout) * time.Second)
	return conn, nil
}

// searchUser searches for a user in LDAP directory
// Returns user DN and email, or error if not found
func (la *LDAPAuthenticator) searchUser(conn *ldap.Conn, username string) (string, string, error) {
	// Build search filter with username, escaping to prevent LDAP injection
	filter := fmt.Sprintf(la.config.UserFilter, ldap.EscapeFilter(username))

	searchRequest := ldap.NewSearchRequest(
		la.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		[]string{"dn", "mail", "email", la.config.UserAttr},
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return "", "", fmt.Errorf("search failed: %w", err)
	}

	if len(sr.Entries) == 0 {
		return "", "", fmt.Errorf("user not found")
	}

	if len(sr.Entries) > 1 {
		return "", "", fmt.Errorf("multiple users found")
	}

	entry := sr.Entries[0]
	dn := entry.DN

	// Try to get email from mail or email attribute
	email := entry.GetAttributeValue("mail")
	if email == "" {
		email = entry.GetAttributeValue("email")
	}

	return dn, email, nil
}
