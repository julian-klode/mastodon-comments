package main

import _ "net/http/pprof"
import "encoding/json"
import "path/filepath"
import "log"
import "net/http"
import "strings"
import "sync"
import "time"
import "strconv"

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
		if status.InReplyToID == nil {
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

func (ct *CommentTool) searchHandler(w http.ResponseWriter, r *http.Request) {
	var result Result

	query := r.FormValue("search")
	if query == "" {
		query = strings.TrimSuffix(r.URL.Path, "comments.json")
	}
	query = filepath.Clean(query)

	roots, err := ct.findToots(query)
	if err != nil {
		w.WriteHeader(503)
		return
	}

	if len(roots) > 0 {
		log.Printf("Querying for comments for %s", query)
		result.Comments, err = ct.getComments(roots[0])
		if err != nil {
			w.WriteHeader(503)
			return
		}
		result.Stats, err = ct.getStatistics(roots[0])
		if err != nil {
			w.WriteHeader(503)
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
		w.WriteHeader(503)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(json)))
	w.Write(json)
}
