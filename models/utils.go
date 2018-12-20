package models

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

func tokenGenerator() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func isValidHostname(host string) (string, bool) {
	valid, _ := regexp.Match("^[a-z0-9]{4,32}$", []byte(host))

	return host, valid
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func DownloadData(url string) (bodyBytes []byte, err error) {
	log.Println("Downloading data from", url)

	rs, err := http.Get(url)
	if err != nil {
		log.Println("Failed to query", url, "-", err)
		return
	}
	defer rs.Body.Close()

	bodyBytes, err = ioutil.ReadAll(rs.Body)
	if err != nil {
		log.Println("Failed to read bytes from request", err)
	}

	return
}
