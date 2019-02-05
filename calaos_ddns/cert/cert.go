package cert

//Using https://github.com/xenolf/lego
//Heavily based on https://github.com/foomo/simplecert

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/xenolf/lego/certificate"
	legoLog "github.com/xenolf/lego/log"
)

// GenerateCert do all the job to obtain a new certicate from letsencrypt
func GenerateCert(domains []string, mail string) (err error) {
	createConfig()

	logFile, err := os.OpenFile(filepath.Join(conf.CacheDir, "letsencrypt.log"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("[FATAL] cert: failed to create logfile", err)
	}

	log.SetOutput(logFile)
	legoLog.Logger = log.New(logFile, "", log.LstdFlags)

	//create acme client
	client, err := createClient(getUser(mail))
	if err != nil {
		return
	}

	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	cert, err := client.Certificate.Obtain(request)
	if err != nil {
		return
	}

	err = saveCertToDisk(cert, conf.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to write cert to disk: %v", err)
	}

	return
}
