package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"time"

	"github.com/coreos/go-systemd/activation"
)

type Config struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func main() {
	var config Config
	if len(os.Args) < 2 {
		log.Panicf("Usage: tool <config file>")
	}

	jsonFile, err := os.Open(os.Args[1])
	if err != nil {
		log.Panicf("Could not open configuration file")
	}

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.Panicf("Could not read configuration file")
	}
	jsonFile.Close()

	err = json.Unmarshal([]byte(byteValue), &config)
	if err != nil {
		log.Panicf("Could not parse configuration file")
	}

	// Setup a custom http transport
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	client := &http.Client{Timeout: time.Second * 5, Transport: transport}
	mastodon := Mastodon{Client: client, Url: config.URL, Token: config.Token}
	ct := CommentTool{mastodon: mastodon}

	listeners, err := activation.Listeners()
	if len(listeners) != 1 {
		log.Panicf("Expected one socket, received %d", len(listeners))
	}

	log.Fatal(fcgi.Serve(listeners[0], http.HandlerFunc(ct.searchHandler)))

}
