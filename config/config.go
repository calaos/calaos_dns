package config

import (
	"github.com/BurntSushi/toml"
)

var (
	Conf Config
)

type Config struct {
	General struct {
		Port           int
		ExpirationDays int `toml:"expiration_days"`
	}
	Powerdns struct {
		Api       string
		ApiKey    string `toml:"api_key"`
		Zone      string
		Blacklist []string
	}
	Database struct {
		Dsn  string
		Type string
	}
}

func ReadConfig(fname string) (err error) {
	if _, err = toml.DecodeFile(fname, &Conf); err != nil {
		return err
	}

	return
}
