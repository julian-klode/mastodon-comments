package main

import _ "net/http/pprof"
import "encoding/json"
import "fmt"
import "io/ioutil"
import "log"
import "net"
import "net/http"
import "os"
import "time"

type Config struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func main() {
	var config Config
	if len(os.Args) < 2 {
		fmt.Println("Usage: tool <config file>")
		os.Exit(1)
	}
	// Open our jsonFile
	jsonFile, err := os.Open(os.Args[1])
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	jsonFile.Close()

	err = json.Unmarshal([]byte(byteValue), &config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	client := &http.Client{Timeout: time.Second * 5, Transport: transport}
	mastodon := Mastodon{Client: client, Url: config.URL, Token: config.Token}
	ct := CommentTool{mastodon: mastodon}

	http.HandleFunc("/search", ct.searchHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
