package e2e

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

// checkDownloadButton hovers the video and expects the Download button, whose card lists file.
// The button is in a closed shadow root, reachable only through the DOM inspector.
func checkDownloadButton(t *testing.T, tab *cdp, file string) {
	var video struct{ X, Y float64 }
	tab.evaluate(t, 0, `(() => { const r = document.querySelector("video").getBoundingClientRect();
		return {x: r.x + r.width / 2, y: r.y + r.height / 2}; })()`, &video)
	tab.call(t, "DOM.enable", nil)
	tab.call(t, "Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": video.X, "y": video.Y})

	pill := tab.waitForNode(t, "the Download button", func(n domNode) bool { return n.hasClass("pill", "on") })
	if icons := pill.count(func(n domNode) bool { return n.NodeName == "svg" }); icons < 3 {
		t.Errorf("the button has %d icons, want its grip, download and close icons", icons)
	}

	open := pill.find(func(n domNode) bool { return n.hasClass("open") })
	if open == nil {
		t.Fatal("the button has no Download part")
	}
	x, y := tab.centerOf(t, open.NodeID)
	for _, kind := range []string{"mousePressed", "mouseReleased"} {
		tab.call(t, "Input.dispatchMouseEvent", map[string]any{"type": kind, "x": x, "y": y, "button": "left", "clickCount": 1})
	}
	cardShows := func(n domNode) bool {
		return n.hasClass("card", "on") && n.find(func(c domNode) bool {
			return c.hasClass("fname") && c.text() == file
		}) != nil
	}
	if !tab.waitFor(t, cardShows) {
		card := tab.document(t).find(func(n domNode) bool { return n.hasClass("card") })
		if card == nil {
			t.Fatal("no card")
		}
		t.Fatalf("the card doesn't list %s; it is %q and says %q", file, card.attribute("class"), card.text())
	}
}

// domNode is a node as the DOM inspector describes it, shadow roots included.
type domNode struct {
	NodeID      int       `json:"nodeId"`
	NodeName    string    `json:"nodeName"`
	NodeValue   string    `json:"nodeValue"`
	Attributes  []string  `json:"attributes"` // name, value, name, value...
	Children    []domNode `json:"children"`
	ShadowRoots []domNode `json:"shadowRoots"`
}

func (n domNode) hasClass(names ...string) bool {
	for i := 0; i+1 < len(n.Attributes); i += 2 {
		if n.Attributes[i] == "class" {
			classes := strings.Fields(n.Attributes[i+1])
			for _, name := range names {
				if !slices.Contains(classes, name) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (n domNode) attribute(name string) string {
	for i := 0; i+1 < len(n.Attributes); i += 2 {
		if n.Attributes[i] == name {
			return n.Attributes[i+1]
		}
	}
	return ""
}

func (n domNode) find(match func(domNode) bool) *domNode {
	if match(n) {
		return &n
	}
	for _, c := range append(slices.Clone(n.ShadowRoots), n.Children...) {
		if found := c.find(match); found != nil {
			return found
		}
	}
	return nil
}

func (n domNode) count(match func(domNode) bool) int {
	total := 0
	if match(n) {
		total++
	}
	for _, c := range append(slices.Clone(n.ShadowRoots), n.Children...) {
		total += c.count(match)
	}
	return total
}

func (n domNode) text() string {
	if n.NodeName == "#text" {
		return n.NodeValue
	}
	var b strings.Builder
	for _, c := range append(slices.Clone(n.ShadowRoots), n.Children...) {
		b.WriteString(c.text())
	}
	return b.String()
}

func (c *cdp) document(t *testing.T) domNode {
	t.Helper()
	var doc struct {
		Root domNode `json:"root"`
	}
	raw := c.call(t, "DOM.getDocument", map[string]any{"depth": -1, "pierce": true})
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Root
}

func (c *cdp) waitFor(t *testing.T, match func(domNode) bool) bool {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if c.document(t).find(match) != nil {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func (c *cdp) waitForNode(t *testing.T, what string, match func(domNode) bool) domNode {
	t.Helper()
	if !c.waitFor(t, match) {
		t.Fatalf("%s never showed", what)
	}
	return *c.document(t).find(match)
}

func (c *cdp) centerOf(t *testing.T, nodeID int) (x, y float64) {
	t.Helper()
	var box struct {
		Model struct {
			Content []float64 `json:"content"` // the corners: x1, y1, ... x4, y4
		} `json:"model"`
	}
	if err := json.Unmarshal(c.call(t, "DOM.getBoxModel", map[string]any{"nodeId": nodeID}), &box); err != nil {
		t.Fatal(err)
	}
	q := box.Model.Content
	if len(q) != 8 {
		t.Fatalf("no box for node %d", nodeID)
	}
	return (q[0] + q[4]) / 2, (q[1] + q[5]) / 2
}
