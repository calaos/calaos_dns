package cert

import (
	"os"
	"time"

	"github.com/calaos/calaos_dns/utils"
)

var conf *Config

// Default contains a default configuration
var Default = &Config{
	// 30 Days before expiration
	RenewBefore: 30 * 24,
	// Once a week
	CheckInterval: 7 * 24 * time.Hour,
	DirectoryURL:  "https://acme-v02.api.letsencrypt.org/directory",
	CacheDirPerm:  0700,
}

// Config allows configuration of simplecert
type Config struct {

	// renew the certificate X hours before it expires
	// LetsEncrypt Certs are valid for 90 Days
	RenewBefore int

	// Interval for checking if cert is closer to expiration than RenewBefore
	CheckInterval time.Duration

	// ACME Directory URL
	DirectoryURL string

	// UNIX Permission for the CacheDir and all files inside
	CacheDirPerm os.FileMode

	// Path of the CacheDir
	CacheDir string
}

// CheckConfig checks if config can be used to obtain a cert
func CreateConfig() {
	c := Default
	c.CacheDir = utils.CreateCacheDir()

	//For testing in dev
	//c.DirectoryURL = "https://acme-staging-v02.api.letsencrypt.org/directory"

	conf = c
}
