package models

import (
	"io"
	"log"
	"net/http"
)

func DownloadData(url string) (bodyBytes []byte, err error) {
	log.Println("Downloading data from", url)

	rs, err := http.Get(url)
	if err != nil {
		log.Println("Failed to query", url, "-", err)
		return
	}
	defer rs.Body.Close()

	bodyBytes, err = io.ReadAll(rs.Body)
	if err != nil {
		log.Println("Failed to read bytes from request", err)
	}

	return
}
