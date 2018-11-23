package main

import "fmt"
import "encoding/json"
import "time"
import "io/ioutil"
import "net/url"
import "net/http"

type Account struct {
	ID             string        `json:"id"`
	Username       string        `json:"username"`
	Acct           string        `json:"acct"`
	DisplayName    string        `json:"display_name"`
	Locked         bool          `json:"locked"`
	Bot            bool          `json:"bot"`
	CreatedAt      time.Time     `json:"created_at"`
	Note           string        `json:"note"`
	URL            string        `json:"url"`
	Avatar         string        `json:"avatar"`
	AvatarStatic   string        `json:"avatar_static"`
	Header         string        `json:"header"`
	HeaderStatic   string        `json:"header_static"`
	FollowersCount int           `json:"followers_count"`
	FollowingCount int           `json:"following_count"`
	StatusesCount  int           `json:"statuses_count"`
	Emojis         []interface{} `json:"emojis"`
	Fields         []struct {
		Name       string      `json:"name"`
		Value      string      `json:"value"`
		VerifiedAt interface{} `json:"verified_at"`
	} `json:"fields"`
}

type Card struct {
	URL          string      `json:"url"`
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	Type         string      `json:"type"`
	AuthorName   string      `json:"author_name"`
	AuthorURL    string      `json:"author_url"`
	ProviderName string      `json:"provider_name"`
	ProviderURL  string      `json:"provider_url"`
	HTML         string      `json:"html"`
	Width        int         `json:"width"`
	Height       int         `json:"height"`
	Image        interface{} `json:"image"`
	EmbedURL     string      `json:"embed_url"`
}

type Status struct {
	ID                 string      `json:"id"`
	CreatedAt          time.Time   `json:"created_at"`
	InReplyToID        *string     `json:"in_reply_to_id"`
	InReplyToAccountID interface{} `json:"in_reply_to_account_id"`
	Sensitive          bool        `json:"sensitive"`
	SpoilerText        string      `json:"spoiler_text"`
	Visibility         string      `json:"visibility"`
	Language           string      `json:"language"`
	URI                string      `json:"uri"`
	Content            string      `json:"content"`
	URL                string      `json:"url"`
	RepliesCount       int         `json:"replies_count"`
	ReblogsCount       int         `json:"reblogs_count"`
	FavouritesCount    int         `json:"favourites_count"`
	Favourited         bool        `json:"favourited"`
	Reblogged          bool        `json:"reblogged"`
	Muted              bool        `json:"muted"`
	Pinned             bool        `json:"pinned,omitempty"`
	Reblog             interface{} `json:"reblog"`
	Application        struct {
		Name    string      `json:"name"`
		Website interface{} `json:"website"`
	} `json:"application"`
	Account          Account       `json:"account"`
	MediaAttachments []interface{} `json:"media_attachments"`
	Mentions         []interface{} `json:"mentions"`
	Tags             []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"tags"`
	Emojis []interface{} `json:"emojis"`
	Card   Card          `json:"card"`
}

type StatusContext struct {
	Descendants []Status `json:"descendants"`
}

type SearchResult struct {
	Hashtags []interface{} `json:"hashtags"`
	Accounts []interface{} `json:"accounts"`
	Statuses []Status      `json:"statuses"`
}

type Mastodon struct {
	Client *http.Client
	Url    string
	Token  string
}

func (m Mastodon) doRequest(method string, values url.Values, result interface{}) error {
	url := fmt.Sprintf("%s/%s", m.Url, method)
	if values != nil {
		url = fmt.Sprintf("%s/%s?%s", m.Url, method, values.Encode())
	}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("bearer %s", m.Token))
	resp, err := m.Client.Do(req)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, result)
}

func (m Mastodon) Search(query string) (SearchResult, error) {
	var result SearchResult
	err := m.doRequest("/api/v1/search", url.Values{
		"q": []string{query},
	}, &result)

	return result, err
}

func (m Mastodon) Statuses(id string) (Status, error) {
	var result Status
	err := m.doRequest(fmt.Sprintf("/api/v1/statuses/%s", id), nil, &result)

	return result, err
}
func (m Mastodon) StatusContext(id string) (StatusContext, error) {
	var result StatusContext
	err := m.doRequest(fmt.Sprintf("/api/v1/statuses/%s/context", id), nil, &result)

	return result, err
}
