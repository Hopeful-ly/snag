package engine

import (
	"slices"
	"strings"
	"time"
)

// Lines of activity kept per download, and the longest a line may be.
const (
	keepSaid    = 60
	maxSaidLine = 400
)

// Said is one line of a download's activity: its program's output or Snag's own notes.
type Said struct {
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

// say records a line of activity. The lock must be held.
func (en *entry) say(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if len(text) > maxSaidLine {
		text = text[:maxSaidLine] + "…"
	}
	en.said = append(en.said, Said{At: time.Now(), Text: text})
	if len(en.said) > keepSaid {
		en.said = slices.Clone(en.said[len(en.said)-keepSaid:])
	}
}

// Activity returns a download's activity since Snag started, oldest first.
func (e *Engine) Activity(id string) ([]Said, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return nil, ErrNotFound
	}
	return slices.Clone(en.said), nil
}
