package cert

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/xenolf/lego/certificate"
)

// CR represents an Lego Certificate Resource
type CR struct {
	Domain            string `json:"domain"`
	CertURL           string `json:"certUrl"`
	CertStableURL     string `json:"certStableUrl"`
	PrivateKey        []byte `json:"privateKey"`
	Certificate       []byte `json:"certificate"`
	IssuerCertificate []byte `json:"issuerCertificate"`
	CSR               []byte `json:"csr"`
}

// get an Lego Certificate Resource from CR
func getACMECertResource(cr *CR) *certificate.Resource {
	var cert = new(certificate.Resource)
	cert.Domain = cr.Domain
	cert.CertURL = cr.CertURL
	cert.CertStableURL = cr.CertStableURL
	cert.PrivateKey = cr.PrivateKey
	cert.Certificate = cr.Certificate
	cert.IssuerCertificate = cr.IssuerCertificate
	cert.CSR = cr.CSR
	return cert
}

func saveCertToDisk(cert *certificate.Resource, cacheDir string) error {

	// JSON encode certificate resource
	// needs to be a CR otherwise the fields with the keys will be lost
	b, err := json.MarshalIndent(CR{
		Domain:            cert.Domain,
		CertURL:           cert.CertURL,
		CertStableURL:     cert.CertStableURL,
		PrivateKey:        cert.PrivateKey,
		Certificate:       cert.Certificate,
		IssuerCertificate: cert.IssuerCertificate,
		CSR:               cert.CSR,
	}, "", "  ")
	if err != nil {
		return err
	}

	// write certificate resource to disk
	err = ioutil.WriteFile(cacheDir+"/CertResource.json", b, conf.CacheDirPerm)
	if err != nil {
		return err
	}

	// write certificate PEM to disk
	err = ioutil.WriteFile(cacheDir+"/cert.pem", cert.Certificate, conf.CacheDirPerm)
	if err != nil {
		return err
	}

	// write private key PEM to disk
	err = ioutil.WriteFile(cacheDir+"/key.pem", cert.PrivateKey, conf.CacheDirPerm)
	if err != nil {
		return err
	}

	fp, err := os.OpenFile(cacheDir+"/cert_key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, conf.CacheDirPerm)
	if err != nil {
		return err
	}
	defer fp.Close()

	_, err = fp.Write(cert.PrivateKey)
	if err != nil {
		return err
	}
	_, err = fp.Write(cert.Certificate)
	if err != nil {
		return err
	}

	return nil
}
