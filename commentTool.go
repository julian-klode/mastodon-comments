/*
 * Copyright (c) 2018 Julian Andres Klode <jak@jak-linux.org>
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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Author struct {
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
	Url         string `json:"url"`
}
type Comment struct {
	Author  Author    `json:"author"`
	Toot    string    `json:"toot"`
	Date    time.Time `json:"date"`
	Url     string    `json:"url"`
	ReplyTo *string   `json:"reply_to"`
	Root    string    `json:"root"`
}

type Stats struct {
	Reblogs int    `json:"reblogs"`
	Favs    int    `json:"favs"`
	Replies int    `json:"replies"`
	Url     string `json:"url"`
	Root    string `json:"root"`
}

type Result struct {
	Comments map[string]Comment `json:"comments"`
	Stats    Stats              `json:"stats"`
}

type CommentTool struct {
	mastodon Mastodon
	roots    sync.Map
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
				Url:         status.Account.URL,
			},
			Toot:    status.Content,
			Date:    status.CreatedAt,
			Url:     status.URI,
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
		Url:     status.URL,
	}
}

func (ct *CommentTool) filterSearchResults(searchResult SearchResult) []string {
	var result []string
	for _, status := range searchResult.Statuses {
		if status.InReplyToID == nil && (ct.userid == "" || status.Account.ID == ct.userid) {
			result = append(result, status.ID)
		}
	}
	return result
}

func (ct *CommentTool) findToots(query string) ([]string, error) {
	if loaded, ok := ct.roots.Load(query); ok {
		return loaded.([]string), nil
	}

	log.Printf("Searching roots for %s", query)

	searchResult, err := ct.mastodon.Search(query)
	if err != nil {
		return nil, err
	}

	result := ct.filterSearchResults(searchResult)
	ct.roots.Store(query, result)
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
