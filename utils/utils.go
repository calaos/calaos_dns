package utils

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/mitchellh/go-homedir"
	"github.com/xenolf/lego/platform/config/env"
)

func CreateCacheDir() string {
	cacheDir := env.GetOrDefaultString("CALAOSDNS_CACHE_DIR", "")
	if cacheDir == "" {
		cacheDir, _ = homedir.Dir()
		cacheDir = cacheDir + "/.cache/calaos/ddns"
	}
	CreateDir(cacheDir, 0700)
	return cacheDir
}

func CreateDir(cacheDir string, cacheperm os.FileMode) {
	_, err := os.Stat(cacheDir)
	if err != nil {
		err = os.MkdirAll(cacheDir, cacheperm)
		if err != nil {
			fmt.Println("Failed to create cache dir.")
		}
	}
}

func TokenGenerator() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func IsValidHostname(host string) (string, bool) {
	valid, _ := regexp.Match("^[a-z0-9]{4,32}$", []byte(host))

	return host, valid
}

func IsValidSubHostname(host string) (string, bool) {
	valid, _ := regexp.Match("^[a-z0-9]{3,32}$", []byte(host))

	return host, valid
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

var (
	globalLogger *log.Logger
	logFile      *os.File
)

func InitLogger() (*log.Logger, *os.File, error) {
	if globalLogger == nil {
		l, err := os.OpenFile(filepath.Join(CreateCacheDir(), "calaos_ddns.log"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return nil, nil, fmt.Errorf("[FATAL] failed to create logfile", err)
		}
		logFile = l

		log.SetOutput(logFile)
		globalLogger = log.New(logFile, "", log.LstdFlags)
	}
	return globalLogger, logFile, nil
}
