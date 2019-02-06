package cert

//Using https://github.com/xenolf/lego
//Heavily based on https://github.com/foomo/simplecert

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/calaos/calaos_dns/utils"
	"github.com/mattn/go-isatty"
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

	err = saveCertToDisk(cert, domains, conf.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to write cert to disk: %v", err)
	}

	return
}

func MaybeRenew(forceRenew bool) (renewdone bool, err error) {
	CreateConfig()

	if isatty.IsTerminal(os.Stdout.Fd()) {
		_, logFile, err := utils.InitLogger()
		if err != nil {
			return false, err
		}
		legoLog.Logger = log.New(logFile, "", log.LstdFlags)
	}

	//load certificate from cache
	if !certCached(conf.CacheDir) {
		return false, errors.New("No certificate in cache found")
	}

	log.Println("Loading cached certificate")

	b, err := ioutil.ReadFile(filepath.Join(conf.CacheDir, "cert.pem"))
	if err != nil {
		log.Printf("[FATAL] cert: failed to read %v from disk: %v", filepath.Join(conf.CacheDir, "cert.pem"), err)
		return
	}

	// Input certificate is PEM encoded. Decode it here as we may need the decoded
	// cert later on in the renewal process. The input may be a bundle or a single certificate.
	certificates, err := parsePEMBundle(b)
	if err != nil {
		log.Println("[FATAL] simplecert: failed to parsePEMBundle: ", err)
		return
	}

	b, err = ioutil.ReadFile(filepath.Join(conf.CacheDir, "CertResource.json"))
	if err != nil {
		log.Println("[FATAL] cert: failed to read CertResource.json from disk: ", err)
		return
	}

	var cr CR
	err = json.Unmarshal(b, &cr)
	if err != nil {
		log.Println("[FATAL] cert: failed to unmarshal certificate resource")
		return
	}

	// check if first cert is CA
	x509Cert := certificates[0]
	if x509Cert.IsCA {
		log.Printf("[%s] Certificate bundle starts with a CA certificate", cr.Domain)
		return false, errors.New("Certificate is a CA")
	}

	// Calculate TimeLeft
	timeLeft := x509Cert.NotAfter.Sub(time.Now().UTC())
	log.Printf("[INFO][%s] acme: %d hours remaining, renewBefore: %d\n", cr.Domain, int(timeLeft.Hours()), int(conf.RenewBefore))

	if int(timeLeft.Hours()) <= int(conf.RenewBefore) || forceRenew {
		log.Printf("Certificate is about to expire, renewing...")

		//create acme client
		client, err := createClient(getUser(""))
		if err != nil {
			return false, err
		}

		request := certificate.ObtainRequest{
			Domains: cr.Domains,
			Bundle:  true,
		}

		cert, err := client.Certificate.Obtain(request)
		if err != nil {
			return false, err
		}

		err = saveCertToDisk(cert, cr.Domains, conf.CacheDir)
		if err != nil {
			err = fmt.Errorf("failed to write cert to disk: %v", err)
		}

		renewdone = true
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
