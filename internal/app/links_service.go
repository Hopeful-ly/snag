package app

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/Roham1299/snag/internal/links"
	"github.com/Roham1299/snag/internal/service"
)

// maxLinksFile is in bytes.
const (
	lookLinksTimeout = time.Minute
	maxLinksFile     = 4 << 20
)

// LinkSet is a set of links found on a page or pasted.
type LinkSet = links.Set

// Link is one link of a set.
type Link = links.Link

// LinksPick is which links of a set to download, and where.
type LinksPick = links.Choice

// LinksAdded is what downloading links did.
type LinksAdded = service.LinksAdded

// LinksView is a set as its window shows it, with its default folder.
type LinksView struct {
	LinkSet
	Folder string `json:"folder"`
}

// LinksService backs the "Download links" window.
type LinksService struct {
	app *App
}

// Get returns the set with id.
func (s *LinksService) Get(id string) (LinksView, error) {
	svc, err := s.app.service()
	if err != nil {
		return LinksView{}, err
	}
	set, err := svc.Links(id)
	if err != nil {
		return LinksView{}, err
	}
	return LinksView{LinkSet: set, Folder: svc.Config().DownloadDir}, nil
}

// Have reports, per link, whether Snag has it already.
func (s *LinksService) Have(id string) ([]bool, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	return svc.LinksHave(id)
}

// Look asks the servers of the links at indexes for their sizes and names.
func (s *LinksService) Look(id string, indexes []int) ([]Link, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), lookLinksTimeout)
	defer cancel()
	return svc.LookAtLinks(ctx, id, indexes)
}

// Download downloads the picked links, then closes the set's window and shows the main one.
func (s *LinksService) Download(id string, pick LinksPick) (LinksAdded, error) {
	svc, err := s.app.service()
	if err != nil {
		return LinksAdded{}, err
	}
	added, err := svc.DownloadLinks(id, pick)
	if err != nil {
		return added, err
	}
	s.app.links.close(id)
	s.app.show()
	return added, nil
}

// AddLinks opens the "Download links" window for the web addresses in pasted text and returns
// the set's id.
func (s *DownloadsService) AddLinks(text string) (string, error) {
	svc, err := s.app.service()
	if err != nil {
		return "", err
	}
	set, err := svc.LinksFromText(text)
	if err != nil {
		return "", err
	}
	s.app.ShowLinks(set.ID)
	return set.ID, nil
}

// ImportLinks opens the "Download links" window for the web addresses in a file the user picks
// (text or a saved page). It returns "" if the user cancels.
func (s *DownloadsService) ImportLinks() (string, error) {
	path, err := s.app.wails.Dialog.OpenFile().
		SetTitle("Import links from a file").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		AddFilter("Text and pages", "*.txt;*.list;*.csv;*.html;*.htm;*.md").
		AddFilter("All files", "*").
		PromptForSingleSelection()
	if err != nil || path == "" {
		return "", nil // cancelled
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxLinksFile))
	if err != nil {
		return "", err
	}
	id, err := s.AddLinks(string(data))
	if errors.Is(err, links.ErrNoLinks) {
		return "", errors.New("that file has no links in it")
	}
	return id, err
}
