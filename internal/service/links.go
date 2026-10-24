package service

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl"
	"github.com/Roham1299/snag/internal/links"
	"github.com/Roham1299/snag/internal/naming"
)

const (
	lookAtOnce  = 4
	lookTimeout = 10 * time.Second
)

// ErrNoLinksSet means the set of links is no longer kept.
var ErrNoLinksSet = errors.New("those links are gone; open them again from their page")

// LinksAdded reports the result of downloading links.
type LinksAdded struct {
	Added int `json:"added"`
	// Already counts links that were already unfinished in the list.
	Already int `json:"already"`
}

// KeepLinks keeps a set of links and returns it with its id.
func (s *Service) KeepLinks(set links.Set) links.Set {
	return s.links.Put(set)
}

// LinksFromText keeps the web addresses in pasted text.
func (s *Service) LinksFromText(text string) (links.Set, error) {
	found := links.InText(text)
	set, err := links.New("", "Pasted links", links.FromPasted, found)
	if err != nil {
		return links.Set{}, err
	}
	return s.links.Put(set), nil
}

// Links returns a set KeepLinks kept.
func (s *Service) Links(id string) (links.Set, error) {
	set, ok := s.links.Get(id)
	if !ok {
		return links.Set{}, ErrNoLinksSet
	}
	return set, nil
}

// LinksHave reports, per link, whether Snag has it unfinished or finished with the file present.
func (s *Service) LinksHave(id string) ([]bool, error) {
	set, err := s.Links(id)
	if err != nil {
		return nil, err
	}
	have := make([]bool, len(set.Links))
	for i, l := range set.Links {
		_, have[i] = s.Engine.Have(engine.HaveQuery{URL: l.URL})
	}
	return have, nil
}

// LookAtLinks asks the servers of the links at indexes for their sizes and names and returns
// the updated links. Video pages aren't asked; a "page" that turns out to be a file is regrouped.
func (s *Service) LookAtLinks(ctx context.Context, id string, indexes []int) ([]links.Link, error) {
	set, err := s.Links(id)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: lookTimeout}
	var wg sync.WaitGroup
	slots := make(chan struct{}, lookAtOnce)
	for _, i := range indexes {
		if i < 0 || i >= len(set.Links) {
			continue
		}
		l := set.Links[i]
		if l.Looked || l.Group == links.VideoPages {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			size, name, page := look(ctx, client, set, l.URL)
			s.links.Learned(id, i, size, name, page)
		}()
	}
	wg.Wait()
	set, err = s.Links(id)
	if err != nil {
		return nil, err
	}
	out := make([]links.Link, 0, len(indexes))
	for _, i := range indexes {
		if i >= 0 && i < len(set.Links) {
			out = append(out, set.Links[i])
		}
	}
	return out, ctx.Err()
}

// look tries HEAD, then a one-byte GET for servers that refuse HEAD.
func look(ctx context.Context, client *http.Client, set links.Set, address string) (size int64, name string, page bool) {
	u, err := url.Parse(address)
	if err != nil {
		return 0, "", false
	}
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequestWithContext(ctx, method, address, nil)
		if err != nil {
			return 0, "", false
		}
		req.Header.Set("User-Agent", httpdl.UserAgent)
		req.Header.Set("Accept-Encoding", "identity")
		for k, v := range set.Headers {
			switch strings.ToLower(k) {
			case "user-agent", "referer":
				req.Header.Set(k, v)
			}
		}
		if cookie := cookies.Header(set.Cookies, u); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		if method == http.MethodGet {
			req.Header.Set("Range", "bytes=0-0")
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return 0, "", false
			}
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			continue
		}
		page = strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") &&
			resp.Header.Get("Content-Disposition") == ""
		if page {
			return 0, "", true
		}
		name = naming.FromContentDisposition(resp.Header.Get("Content-Disposition"))
		if name == "" && resp.Request.URL.String() != address {
			name = naming.FromAddress(resp.Request.URL.String(), false) // where it redirected to
		}
		if name != "" {
			name = naming.Safe(naming.WithExtension(name, resp.Header.Get("Content-Type")), "")
		}
		switch {
		case resp.StatusCode == http.StatusPartialContent:
			if _, total, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok {
				size, _ = strconv.ParseInt(strings.TrimSpace(total), 10, 64)
			}
		case method == http.MethodHead && resp.ContentLength > 0:
			size = resp.ContentLength
		}
		return size, name, false
	}
	return 0, "", false
}

// DownloadLinks downloads the picked links to the chosen folder and queue, with no Save window.
func (s *Service) DownloadLinks(id string, c links.Choice) (LinksAdded, error) {
	set, err := s.Links(id)
	if err != nil {
		return LinksAdded{}, err
	}
	if c.Queue == "" {
		c.Queue = config.MainQueue
	} else if _, ok := s.Config().Queue(c.Queue); !ok {
		return LinksAdded{}, errors.New("that queue is gone")
	}
	c.Directory = expandHome(strings.TrimSpace(c.Directory))
	reqs, err := links.Plan(set, c)
	if err != nil {
		return LinksAdded{}, err
	}
	added, already, err := s.Engine.AddMany(reqs)
	if err != nil {
		return LinksAdded{}, err
	}
	if _, err := s.Update(func(cfg *config.Config) { cfg.AddRecentDir(c.Directory) }); err != nil {
		s.log.Warn("couldn't remember the folder", "err", err)
	}
	return LinksAdded{Added: added, Already: already}, nil
}

// ShowLinks opens the links window, if there is a window.
func (s *Service) ShowLinks(id string) {
	s.mu.Lock()
	w := s.window
	s.mu.Unlock()
	if w != nil {
		w.ShowLinks(id)
	}
}
