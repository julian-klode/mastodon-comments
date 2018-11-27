/*
 * Copyright (c) 2018 Julian Andres Klode <jak@jak-linux.org>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
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
	Userid string `json:"userid"`
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
	ct := CommentTool{mastodon: mastodon, userid: config.Userid}

	listeners, err := activation.Listeners()
	if len(listeners) != 1 {
		log.Panicf("Expected one socket, received %d", len(listeners))
	}

	log.Fatal(fcgi.Serve(listeners[0], &ct))

}
