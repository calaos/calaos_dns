package calaos

import (
	"encoding/xml"
	"io/ioutil"
	"log"
	"os"

	"github.com/calaos/calaos_dns/utils"
	"github.com/mitchellh/go-homedir"
	"github.com/xenolf/lego/platform/config/env"
)

var (
	xml_file   string
	configBase string
)

const (
	LOCAL_CONFIG     = "local_config.xml"
	HOME_CONFIG_PATH = ".config/calaos"
	ETC_CONFIG_PATH  = "/etc/calaos"
)

func getDefaultConfig() (d string) {
	home, _ := homedir.Dir()
	confDirs := []string{
		home + "/" + HOME_CONFIG_PATH,
		ETC_CONFIG_PATH,
	}

	for _, d = range confDirs {
		conf := d + "/" + LOCAL_CONFIG
		if _, err := os.Stat(conf); !os.IsNotExist(err) {
			return
		}
	}

	log.Println("ERROR: Config file not found in any known dir!")

	return
}

func getConfigFile(file string) string {
	if configBase == "" {
		configBase = env.GetOrDefaultString("CALAOS_CONFIG", getDefaultConfig())
	}

	return configBase + "/" + file
}

type CalaosConfig struct {
	XMLName xml.Name       `xml:"config"`
	Options []CalaosOption `xml:"option"`
}

type CalaosOption struct {
	XMLName xml.Name `xml:"option"`
	Key     string   `xml:"name,attr"`
	Value   string   `xml:"value,attr"`
}

func GetConfig(key string) (err error, value string) {
	xmlFile, err := os.Open(getConfigFile(LOCAL_CONFIG))
	if err != nil {
		log.Println("Failed to open file:", err)
		return
	}
	defer xmlFile.Close()

	byteValue, _ := ioutil.ReadAll(xmlFile)
	conf := CalaosConfig{}
	xml.Unmarshal(byteValue, &conf)

	for _, opt := range conf.Options {
		if opt.Key == key {
			return nil, opt.Value
		}
	}

	return
}

func SetConfig(key, value string) (err error) {
	xmlFile, err := os.OpenFile(getConfigFile(LOCAL_CONFIG), os.O_RDWR, 0666)
	if err != nil {
		log.Println("Failed to open file:", err)
		return
	}
	defer xmlFile.Close()

	byteValue, _ := ioutil.ReadAll(xmlFile)
	conf := CalaosConfig{}
	xml.Unmarshal(byteValue, &conf)

	isSet := false

	out := `<?xml version="1.0" encoding="UTF-8" ?>
<calaos:config xmlns:calaos="http://www.calaos.fr">
`

	for _, opt := range conf.Options {
		v := opt.Value
		if opt.Key == key {
			v = value
			isSet = true
		}

		out += "    <calaos:option name=\"" + opt.Key + "\" value=\"" + v + "\" />\n"
	}

	if !isSet {
		out += "    <calaos:option name=\"" + key + "\" value=\"" + value + "\" />\n"
	}

	out += `</calaos:config>`

	xmlFile.Truncate(0)
	xmlFile.Seek(0, 0)

	_, err = xmlFile.WriteString(out)

	return
}

func DeleteConfig(key string) (err error) {
	xmlFile, err := os.OpenFile(getConfigFile(LOCAL_CONFIG), os.O_RDWR, 0666)
	if err != nil {
		log.Println("Failed to open file:", err)
		return
	}
	defer xmlFile.Close()

	byteValue, _ := ioutil.ReadAll(xmlFile)
	conf := CalaosConfig{}
	xml.Unmarshal(byteValue, &conf)

	out := `<?xml version="1.0" encoding="UTF-8" ?>
<calaos:config xmlns:calaos="http://www.calaos.fr">
`

	for _, opt := range conf.Options {
		v := opt.Value
		if opt.Key == key {
			continue
		}

		out += "    <calaos:option name=\"" + opt.Key + "\" value=\"" + v + "\" />\n"
	}

	out += `</calaos:config>`

	xmlFile.Truncate(0)
	xmlFile.Seek(0, 0)

	_, err = xmlFile.WriteString(out)

	return
}

func init() {
	utils.InitLogger()
	log.Println("Using file:", getConfigFile(LOCAL_CONFIG))
}
