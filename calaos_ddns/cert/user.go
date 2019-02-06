package cert

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/xenolf/lego/registration"
)

// SSLUser implements the ACME User interface
type SSLUser struct {
	Email        string
	Registration *registration.Resource
	Key          *rsa.PrivateKey
}

func (u SSLUser) GetEmail() string {
	return u.Email
}

func (u SSLUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u SSLUser) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}

// get SSL User from cacheDir or create a new one
func getUser(email string) SSLUser {

	// no cached cert. start from scratch
	var u SSLUser

	// do we have a user?
	b, err := ioutil.ReadFile(conf.CacheDir + "/SSLUser.json")
	if err == nil {
		// user exists. load
		err = json.Unmarshal(b, &u)
		if err != nil {
			fmt.Println("[FATAL] cert: failed to unmarshal SSLUser: ", err)
		}
	} else {
		// create private key
		privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			log.Fatal(err)
		}

		if email == "" {
			log.Fatal("[acme] Empty email address")
		}

		// Create new user
		u = SSLUser{
			Email: email,
			Key:   privateKey,
		}
	}

	return u
}

// save the user on disk
// fatals on error
func saveUserToDisk(u SSLUser, cacheDir string) {
	b, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		fmt.Println("[FATAL] cert: failed to marshal user: ", err)
	}
	err = ioutil.WriteFile(conf.CacheDir+"/SSLUser.json", b, conf.CacheDirPerm)
	if err != nil {
		fmt.Println("[FATAL] cert: failed to write user to disk: ", err)
	}
}
