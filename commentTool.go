/*
 * Copyright (c) 2018-2019 Julian Andres Klode <jak@jak-linux.org>
 *
 * Parts derived from the getcomment.php script provided in
 * https://gitlab.com/BeS/hugo-sustain-ng/ which is
 *   Copyright (c) 2018 Bjoern Schiessle <bjoern@schiessle.org>
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
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Author is a person on mastodon
type Author struct {
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
	URL         string `json:"url"`
}

// Comment is a reply a person made to a toot
type Comment struct {
	Author  Author    `json:"author"`
	Toot    string    `json:"toot"`
	Date    time.Time `json:"date"`
	URL     string    `json:"url"`
	ReplyTo *string   `json:"reply_to"`
	Root    string    `json:"root"`
}

// Stats holds statistics about a toot
type Stats struct {
	Reblogs int    `json:"reblogs"`
	Favs    int    `json:"favs"`
	Replies int    `json:"replies"`
	URL     string `json:"url"`
	Root    string `json:"root"`
}

// Result is the representation of comments for a given toot that
// CommentTool will serve in response to an http request.
type Result struct {
	Comments map[string]Comment `json:"comments"`
	Stats    Stats              `json:"stats"`
}

// State contains all the roots we know about
type State struct {
	Roots    map[string][]string `json:"roots"`
	mutex    sync.RWMutex
	filename string
}

// LoadState loads the roots database
func LoadState(filename string) *State {
	state := &State{Roots: make(map[string][]string), filename: filename}
	jsonFile, err := os.Open(state.filename)
	if err == nil {
		defer jsonFile.Close()

		err = json.NewDecoder(jsonFile).Decode(state)
		if err != nil {
			log.Printf("Could not decode roots file: %s", err)
		}
	} else {
		log.Printf("Could not open roots file: %s", err)
	}
	return state
}

// Get looks up a specific key
func (state *State) Get(key string) ([]string, bool) {
	state.mutex.RLock()
	value, ok := state.Roots[key]
	state.mutex.RUnlock()
	return value, ok
}

// Put puts in a key and causes a writeout
func (state *State) Put(key string, value []string) {
	state.mutex.Lock()
	state.Roots[key] = value
	state.mutex.Unlock()

	go state.writeout()
}

// writeout writes the file back
func (state *State) writeout() {
	state.mutex.RLock()
	defer state.mutex.RUnlock()

	log.Println("Writing root state")

	jsonFile, err := os.Create(state.filename + ".new")
	if err != nil {
		log.Printf("Could not open state file: %s", err)
		return
	}
	defer jsonFile.Close()

	err = json.NewEncoder(jsonFile).Encode(state)
	if err != nil {
		log.Printf("Could not write state file: %s", err)
		return
	}

	jsonFile.Close()
	if err := os.Rename(jsonFile.Name(), state.filename); err != nil {
		log.Printf("Could not commit state file: %s", err)
		return
	}
}

// CommentTool is an HTTP service
type CommentTool struct {
	mastodon Mastodon
	roots    *State
	userid   string
}

func (ct *CommentTool) filterComments(statuses []Status, root string) map[string]Comment {
	comments := make(map[string]Comment)
	for _, status := range statuses {
		name := status.Account.DisplayName
		if name == "" {
			name = status.Account.Username
		}
		comments[status.ID] = Comment{
			Author: Author{
				DisplayName: name,
				Avatar:      status.Account.AvatarStatic,
				URL:         status.Account.URL,
			},
			Toot:    status.Content,
			Date:    status.CreatedAt,
			URL:     status.URI,
			ReplyTo: status.InReplyToID,
			Root:    root,
		}

	}

	return comments

}
func (ct *CommentTool) filterStats(status Status) Stats {
	return Stats{
		Reblogs: status.ReblogsCount,
		Favs:    status.FavouritesCount,
		Replies: status.RepliesCount,
		URL:     status.URL,
	}
}

func (ct *CommentTool) filterSearchResults(searchResult SearchResult, query string) []string {
	var result []string
	for _, status := range searchResult.Statuses {
		if status.InReplyToID == nil && (ct.userid == "" || status.Account.ID == ct.userid) && strings.Contains(status.Content, query) {
			result = append(result, status.ID)
		}
	}
	return result
}

func (ct *CommentTool) findToots(query string) ([]string, error) {
	if loaded, ok := ct.roots.Get(query); ok {
		return loaded, nil
	}

	log.Printf("Searching roots for %s", query)

	searchResult, err := ct.mastodon.Search(query)
	if err != nil {
		return nil, err
	}

	result := ct.filterSearchResults(searchResult, query)
	ct.roots.Put(query, result)
	return result, nil
}

func (ct *CommentTool) getComments(id string) (map[string]Comment, error) {
	ctx, err := ct.mastodon.StatusContext(id)
	if err != nil {
		return nil, err
	}

	return ct.filterComments(ctx.Descendants, id), nil
}

func (ct *CommentTool) getStatistics(id string) (Stats, error) {
	status, err := ct.mastodon.Statuses(id)
	if err != nil {
		return Stats{}, err
	}

	return ct.filterStats(status), nil
}

func (ct *CommentTool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var result Result

	query := r.FormValue("search")
	if query == "" {
		query = strings.TrimSuffix(r.URL.Path, "comments.json")
	}
	query = filepath.Clean(query)

	roots, err := ct.findToots(query)
	if err != nil {
		log.Printf("ERROR: Could not determine roots: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(roots) > 0 {
		log.Printf("Querying for comments for %s", query)
		result.Comments, err = ct.getComments(roots[0])
		if err != nil {
			log.Printf("ERROR: Could not query comments: %s", err)
			w.WriteHeader(500)
			return
		}
		result.Stats, err = ct.getStatistics(roots[0])
		if err != nil {
			log.Printf("ERROR: Could not get statistics: %s", err)
			w.WriteHeader(500)
			return
		}
		result.Stats.Root = roots[0]
		result.Stats.Replies = len(result.Comments)
		w.Header().Set("Cache-Control", "max-age=600")
	} else {
		log.Printf("No roots found for %s", query)
		w.Header().Set("Cache-Control", "max-age=60")
	}

	json, err := json.Marshal(result)

	if err != nil {
		log.Printf("ERROR: Could not marshal result: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(json)))
	w.Write(json)
}
