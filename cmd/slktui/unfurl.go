package main

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	slkdb "github.com/stevejackson/slk/internal/db"
)

var xURLRE = regexp.MustCompile(`https?://(?:x\.com|twitter\.com)/\S+`)

type unfurledMsg struct {
	ts      string
	unfurls map[string]string // url → tweet text
}

// fetchUnfurls returns a Cmd that fetches oEmbed content for all X URLs in the thread.
func fetchUnfurls(t slkdb.Thread) tea.Cmd {
	return func() tea.Msg {
		urls := extractXURLs(t)
		if len(urls) == 0 {
			return nil
		}
		unfurls := make(map[string]string)
		client := &http.Client{Timeout: 5 * time.Second}
		for _, u := range urls {
			if text := oEmbed(client, u); text != "" {
				unfurls[u] = text
			}
		}
		if len(unfurls) == 0 {
			return nil
		}
		return unfurledMsg{ts: t.TS, unfurls: unfurls}
	}
}

func extractXURLs(t slkdb.Thread) []string {
	seen := map[string]bool{}
	var urls []string
	for _, raw := range xURLRE.FindAllString(t.Text, -1) {
		u := cleanXURL(raw)
		if !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	for _, r := range t.Replies {
		for _, raw := range xURLRE.FindAllString(r.Text, -1) {
			u := cleanXURL(raw)
			if !seen[u] {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	return urls
}

// cleanXURL strips trailing punctuation and markdown artifacts.
func cleanXURL(raw string) string {
	raw = strings.TrimRight(raw, ".,)\"'")
	// strip query params that are tracking-only (s=, t=)
	if u, err := url.Parse(raw); err == nil {
		q := u.Query()
		q.Del("s")
		q.Del("t")
		u.RawQuery = q.Encode()
		raw = u.String()
	}
	return raw
}

type oEmbedResponse struct {
	HTML       string `json:"html"`
	AuthorName string `json:"author_name"`
}

func oEmbed(client *http.Client, tweetURL string) string {
	endpoint := "https://publish.twitter.com/oembed?omit_script=true&url=" + url.QueryEscape(tweetURL)
	resp, err := client.Get(endpoint)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	defer resp.Body.Close()

	var oe oEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&oe); err != nil {
		return ""
	}
	return extractTweetText(oe.HTML, oe.AuthorName)
}

// inlineUnfurls replaces X URLs in text with their fetched tweet content.
func inlineUnfurls(text string, unfurls map[string]string) string {
	if len(unfurls) == 0 {
		return text
	}
	return xURLRE.ReplaceAllStringFunc(text, func(raw string) string {
		u := cleanXURL(raw)
		if tweet, ok := unfurls[u]; ok {
			return raw + "\n> " + strings.ReplaceAll(tweet, "\n", "\n> ")
		}
		return raw
	})
}

// extractTweetText pulls the tweet body out of oEmbed HTML.
// oEmbed HTML looks like: <blockquote ...><p ...>TEXT <a>...</a></p>&mdash; Author ...</blockquote>
var pTagRE = regexp.MustCompile(`(?s)<p[^>]*>(.*?)</p>`)
var tagRE = regexp.MustCompile(`<[^>]+>`)

func extractTweetText(rawHTML, author string) string {
	m := pTagRE.FindStringSubmatch(rawHTML)
	if m == nil {
		return ""
	}
	text := tagRE.ReplaceAllString(m[1], "")
	text = html.UnescapeString(text)
	text = strings.TrimSpace(text)
	if author != "" {
		return text + "\n  — " + author
	}
	return text
}
