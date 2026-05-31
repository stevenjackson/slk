package main

import (
	"regexp"
	"strings"

	slkdb "github.com/stevejackson/slk/internal/db"
)

// noiseScore returns a value from 0.0 (strong signal) to 1.0 (strong noise).
func noiseScore(t slkdb.Thread) float64 {
	text := strings.TrimSpace(t.Text)

	// --- certain noise ---

	// Automated bot messages
	if strings.EqualFold(t.Author, "slackbot") {
		return 1.0
	}

	// Image-only: no real text, just a file attachment
	if isImageOnly(text) {
		return 1.0
	}

	// Cross-post: Slack internal link with no surrounding context
	if isCrossPost(text) {
		return 0.95
	}

	// Joke/meme thread: laugh indicators in replies even if root post looks neutral
	if isJokeThread(t) {
		return 0.85
	}

	// --- accumulate score ---

	score := 0.0

	// 0 replies — meaningful noise signal
	if t.ReplyCount == 0 {
		score += 0.35
	}

	// Bare link drop: short text that's mostly a URL + optional attachment title
	if isBareLinkDrop(text) {
		score += 0.3
	}

	// Single-line reaction / very short text with no URL
	if isSingleLineReaction(text) {
		score += 0.40
	}

	// --- signal reducers ---

	// High reply count
	if t.ReplyCount >= 10 {
		score -= 0.5
	} else if t.ReplyCount >= 5 {
		score -= 0.15
	}

	// PR announcement
	if prAnnouncementRE.MatchString(text) {
		score -= 0.3
	}

	return clamp(score, 0.0, 1.0)
}

// noiseLabel returns a colored indicator string for the list view.
func noiseLabel(score float64) string {
	switch {
	case score >= 0.7:
		return dimStyle.Render("○") // likely noise — dim
	case score >= 0.4:
		return replyCountStyle.Render("◑") // uncertain — yellow
	default:
		return channelStyle.Render("●") // likely signal — blue
	}
}

// --- classifiers ---

var imageFileRE = regexp.MustCompile(`(?i)\[file:[^\]]+image/[^\]]+\]`)

func isImageOnly(text string) bool {
	// Text is empty or contains only file attachment markers
	stripped := imageFileRE.ReplaceAllString(text, "")
	return strings.TrimSpace(stripped) == ""
}

var slackInternalRE = regexp.MustCompile(`https://[a-z]+\.slack\.com/archives/`)

func isCrossPost(text string) bool {
	if !slackInternalRE.MatchString(text) {
		return false
	}
	// Low surrounding context: strip the URL and see if much is left
	stripped := slackInternalRE.ReplaceAllString(text, "")
	stripped = regexp.MustCompile(`\[attachment:[^\]]+\]`).ReplaceAllString(stripped, "")
	return len(strings.TrimSpace(stripped)) < 30
}

var urlRE = regexp.MustCompile(`https?://\S+`)
var attachmentRE = regexp.MustCompile(`\[attachment:[^\]]+\]`)

func isBareLinkDrop(text string) bool {
	// Strip URLs and attachment titles, see if meaningful text remains
	stripped := urlRE.ReplaceAllString(text, "")
	stripped = attachmentRE.ReplaceAllString(stripped, "")
	stripped = strings.TrimSpace(stripped)
	// If remaining text is very short, it was mostly a link drop
	return len(stripped) < 25
}

func isSingleLineReaction(text string) bool {
	if strings.Contains(text, "\n") {
		return false
	}
	if urlRE.MatchString(text) {
		return false
	}
	return len(text) < 80
}

var jokeRE = regexp.MustCompile(`(?i)(lol|:laughing:|:rolling_on_the_floor_laughing:|:joy:|the onion|joke|lmao|haha|:rofl:)`)

// isJokeThread returns true when reply content signals a meme/joke thread
// even if the root post looks neutral.
func isJokeThread(t slkdb.Thread) bool {
	jokeCount := 0
	for _, r := range t.Replies {
		if jokeRE.MatchString(r.Text) {
			jokeCount++
		}
	}
	return jokeCount >= 2
}

var prAnnouncementRE = regexp.MustCompile(`(?i)(opened a PR|pull request)`)

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
