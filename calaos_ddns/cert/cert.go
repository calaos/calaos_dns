package cert

//Using https://github.com/xenolf/lego
//Heavily based on https://github.com/foomo/simplecert

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/calaos/calaos_dns/utils"
	"github.com/xenolf/lego/certificate"
	legoLog "github.com/xenolf/lego/log"
)

// GenerateCert do all the job to obtain a new certicate from letsencrypt
func GenerateCert(domains []string, mail string) (err error) {
	CreateConfig()

	_, logFile, err := utils.InitLogger()
	if err != nil {
		return
	}
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

func WritePemFile(certfile string) (err error) {
	in, err := os.Open(conf.CacheDir + "/cert_key.pem")
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.OpenFile(certfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
