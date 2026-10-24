package config

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/naming"
)

// MainQueue is the default queue's ID; its concurrency is Config.MaxActive.
const MainQueue = "main"

// WhenDone actions, run once a queue has finished everything in it.
const (
	WhenDoneNothing  = ""
	WhenDoneSleep    = "sleep"
	WhenDoneShutdown = "shutdown"
	WhenDoneQuit     = "quit"
)

// MaxQueues is the limit on queues besides Main.
const MaxQueues = 20

// Retry defaults and limits; waits are in seconds.
const (
	DefaultRetries   = 3
	DefaultRetryWait = 5
	maxRetries       = 20
	maxRetryWait     = 300
)

// Queue is a download queue with its own order, concurrency and optional schedule.
type Queue struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	AtOnce int    `json:"atOnce"`
	// Schedule is nil to run when started (always so for Main).
	Schedule Schedule `json:"schedule"`
	// SpeedLimit is in bytes/s across the queue; 0 for none.
	SpeedLimit int64  `json:"speedLimit,omitempty"`
	WhenDone   string `json:"whenDone,omitempty"`
	// Naming is "" for the settings' default.
	Naming  naming.Template `json:"naming,omitempty"`
	Retries int             `json:"retries"`
	// RetryWait is in seconds.
	RetryWait int `json:"retryWait"`
	// Fallback is "youtube" or "soundcloud" for songs YouTube Music lacks; "" asks each time.
	Fallback string `json:"fallback,omitempty"`
}

// UnmarshalJSON applies the retry defaults to queues saved without retry fields; an explicit 0 stays.
func (q *Queue) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	type queue Queue
	var plain queue
	if err := json.Unmarshal(data, &plain); err != nil {
		return err
	}
	*q = Queue(plain)
	if _, ok := raw["retries"]; !ok {
		q.Retries = DefaultRetries
	}
	if _, ok := raw["retryWait"]; !ok {
		q.RetryWait = DefaultRetryWait
	}
	return nil
}

// Schedule is when a queue runs by itself.
type Schedule struct {
	On    bool   `json:"on"`
	Start string `json:"start"`          // "15:04"
	Stop  string `json:"stop,omitempty"` // "" to run until done
	// Days are time.Weekday values; none means every day.
	Days []int `json:"days,omitempty"`
}

// ClockTime reads a time of day written "15:04" as minutes since midnight.
func ClockTime(s string) (int, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// Queue returns the queue with id.
func (c Config) Queue(id string) (Queue, bool) {
	i := slices.IndexFunc(c.Queues, func(q Queue) bool { return q.ID == id })
	if i < 0 {
		return Queue{}, false
	}
	return c.Queues[i], true
}

// SetQueue adds q, or replaces the queue with its id.
func (c *Config) SetQueue(q Queue) {
	if q.ID == MainQueue {
		c.MaxActive = q.AtOnce
	}
	if i := slices.IndexFunc(c.Queues, func(old Queue) bool { return old.ID == q.ID }); i >= 0 {
		c.Queues[i] = q
		return
	}
	c.Queues = append(c.Queues, q)
}

// ArrangeQueues orders the queues by ids. Main stays first; queues missing from ids go last.
func (c *Config) ArrangeQueues(ids []string) {
	place := map[string]int{MainQueue: -1}
	for i, id := range ids {
		if _, seen := place[id]; !seen {
			place[id] = i
		}
	}
	at := func(q Queue) int {
		if i, ok := place[q.ID]; ok {
			return i
		}
		return len(ids)
	}
	slices.SortStableFunc(c.Queues, func(a, b Queue) int { return cmp.Compare(at(a), at(b)) })
}

// RemoveQueue removes a queue; Main can't be removed.
func (c *Config) RemoveQueue(id string) {
	if id == MainQueue {
		return
	}
	c.Queues = slices.DeleteFunc(c.Queues, func(q Queue) bool { return q.ID == id })
}

// normalQueues puts Main first, clamps settings, and drops queues with empty or duplicate ids.
func (c Config) normalQueues() []Queue {
	main := Queue{ID: MainQueue, Name: "Main", Retries: DefaultRetries, RetryWait: DefaultRetryWait}
	var others []Queue
	seen := map[string]bool{}
	for _, q := range c.Queues {
		switch {
		case q.ID == MainQueue:
			main = q
		case q.ID != "" && !seen[q.ID] && len(others) < MaxQueues:
			seen[q.ID] = true
			others = append(others, q)
		}
	}
	main.AtOnce = c.MaxActive
	queues := []Queue{normalQueue(main)}
	for _, q := range others {
		queues = append(queues, normalQueue(q))
	}
	return queues
}

func normalQueue(q Queue) Queue {
	q.Name = strings.TrimSpace(q.Name)
	if q.Name == "" {
		q.Name = "Queue"
	}
	q.AtOnce = clamp(q.AtOnce, 1, maxActive)
	q.Retries = clamp(q.Retries, 0, maxRetries)
	q.RetryWait = clamp(q.RetryWait, 0, maxRetryWait)
	q.SpeedLimit = max(q.SpeedLimit, 0)
	q.Naming = naming.Template(strings.TrimSpace(string(q.Naming)))
	if q.Naming.Check() != nil {
		q.Naming = ""
	}
	switch q.WhenDone {
	case WhenDoneNothing, WhenDoneSleep, WhenDoneShutdown, WhenDoneQuit:
	default:
		q.WhenDone = WhenDoneNothing
	}
	switch q.Fallback {
	case "", "youtube", "soundcloud":
	default:
		q.Fallback = ""
	}
	s := &q.Schedule
	if _, ok := ClockTime(s.Start); !ok {
		s.On, s.Start = false, ""
	}
	if _, ok := ClockTime(s.Stop); !ok || s.Stop == s.Start {
		s.Stop = ""
	}
	var days []int
	for _, d := range s.Days {
		if d >= 0 && d <= 6 && !slices.Contains(days, d) {
			days = append(days, d)
		}
	}
	slices.Sort(days)
	if len(days) == 7 {
		days = nil
	}
	s.Days = days
	return q
}
