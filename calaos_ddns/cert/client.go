package cert

import (
	"fmt"

	"github.com/calaos/calaos_dns/calaos_ddns/cert/calaosdns"
	"github.com/xenolf/lego/certcrypto"
	"github.com/xenolf/lego/lego"
	"github.com/xenolf/lego/registration"
)

func createClient(u SSLUser) (client *lego.Client, err error) {

	c := lego.NewConfig(&u)
	c.CADirURL = conf.DirectoryURL
	c.Certificate.KeyType = certcrypto.RSA4096

	// Create a new client instance
	client, err = lego.NewClient(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v, %v", err, c)
	}

	// Setup DNS challenge
	provider, err := calaosdns.NewDNSProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to create DNS provider: %v", err)
	}

	err = client.Challenge.SetDNS01Provider(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to set DNS provider: %v", err)
	}

	// register if necessary
	if u.Registration == nil {
		// Register Client
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, fmt.Errorf("failed to register client: %v", err)
		}
		u.Registration = reg

		saveUserToDisk(u, conf.CacheDir)
	}

	return
}
