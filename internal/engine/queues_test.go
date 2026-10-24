package engine

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Two queues: Main, one at a time, and Tonight, two at a time, which waits to be started.
func queuesEngine(t *testing.T) *testEngine {
	return newTestEngine(t, Settings{MaxActive: 1, Queues: []QueueSettings{
		{ID: MainQueue, AtOnce: 1},
		{ID: "night", AtOnce: 2},
	}})
}

func addr(name string) string { return "https://example.com/" + name }

var errNope = errors.New("nope")

func (te *testEngine) statusOf(t *testing.T, id string) Status {
	t.Helper()
	d, err := te.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return d.Status
}

// staysQueued checks that downloads wait, a moment after anything could have started them.
func (te *testEngine) staysQueued(t *testing.T, ds ...Download) {
	t.Helper()
	time.Sleep(30 * time.Millisecond)
	for _, d := range ds {
		if s := te.statusOf(t, d.ID); s != StatusQueued {
			t.Errorf("%s is %s, want waiting", d.URL, s)
		}
	}
}

func TestEachQueueRunsItsOwnDownloadsAtOnce(t *testing.T) {
	te := queuesEngine(t)
	a := te.add(t, AddRequest{URL: addr("a")})
	b := te.add(t, AddRequest{URL: addr("b")})
	c := te.add(t, AddRequest{URL: addr("c"), Queue: "night"})
	d := te.add(t, AddRequest{URL: addr("d"), Queue: "night"})
	e := te.add(t, AddRequest{URL: addr("e"), Queue: "night"})

	te.waitFor(t, a.ID, StatusDownloading)
	te.staysQueued(t, b, c, d, e) // Main runs one at a time; Tonight hasn't started

	te.StartQueue("night")
	te.waitQueueEvent(t, QueueEvent{QueueStarted, "night"})
	te.waitFor(t, c.ID, StatusDownloading)
	te.waitFor(t, d.ID, StatusDownloading)
	te.staysQueued(t, b, e)
	if !te.QueueRunning("night") || !te.QueueRunning(MainQueue) {
		t.Error("the queues don't say they run")
	}

	te.runner.finish(c.URL, nil)
	te.waitFor(t, e.ID, StatusDownloading) // next in Tonight's line
	te.staysQueued(t, b)
}

func TestAQueueRunsInTheOrderItsGiven(t *testing.T) {
	te := queuesEngine(t)
	var ds []Download
	for _, name := range []string{"1", "2", "3", "4"} {
		ds = append(ds, te.add(t, AddRequest{URL: addr(name), Queue: "night"}))
	}
	// Dragged: 4 to the top, then 3 before 1.
	if err := te.Reorder("night", []string{ds[3].ID, ds[2].ID, ds[0].ID, ds[1].ID}); err != nil {
		t.Fatal(err)
	}
	te.StartQueue("night")
	te.waitFor(t, ds[3].ID, StatusDownloading)
	te.waitFor(t, ds[2].ID, StatusDownloading)
	te.staysQueued(t, ds[0], ds[1])

	if err := te.ToFront(ds[1].ID); err != nil {
		t.Fatal(err)
	}
	te.runner.finish(ds[3].URL, nil)
	te.waitFor(t, ds[1].ID, StatusDownloading)
	te.staysQueued(t, ds[0])
}

func TestReverseFlipsTheLine(t *testing.T) {
	te := queuesEngine(t)
	var ds []Download
	for _, name := range []string{"1", "2", "3", "4"} {
		ds = append(ds, te.add(t, AddRequest{URL: addr(name), Queue: "night", Paused: true}))
	}
	events, unsubscribe := te.Subscribe()
	defer unsubscribe()

	te.Reverse("night")
	var order []string
	list := te.List()
	slices.SortFunc(list, func(a, b Download) int { return int(a.Position - b.Position) })
	for _, d := range list {
		if d.Queue == "night" {
			order = append(order, d.URL)
		}
	}
	if want := []string{addr("4"), addr("3"), addr("2"), addr("1")}; !slices.Equal(order, want) {
		t.Errorf("order %v, want %v", order, want)
	}
	got := 0
	deadline := time.After(time.Second)
	for got < 1 {
		select {
		case ev := <-events:
			got++
			if ev.Kind != EventReset {
				t.Errorf("event %s, want one reset", ev.Kind)
			}
		case <-deadline:
			t.Fatal("no reset")
		}
	}

	te.StartQueue("night")
	if err := te.Resume(ds[3].ID); err != nil {
		t.Fatal(err)
	}
	te.waitFor(t, ds[3].ID, StatusDownloading) // the one that was last
}

func TestReorderKeepsTheRestOfTheLine(t *testing.T) {
	te := queuesEngine(t)
	var ds []Download
	for _, name := range []string{"1", "2", "3", "4"} {
		ds = append(ds, te.add(t, AddRequest{URL: addr(name), Queue: "night", Paused: true}))
	}
	if err := te.Reorder("night", []string{ds[3].ID, ds[1].ID}); err != nil {
		t.Fatal(err)
	}
	var order []string
	list := te.List()
	slices.SortFunc(list, func(a, b Download) int { return int(a.Position - b.Position) })
	for _, d := range list {
		order = append(order, d.URL)
	}
	if want := []string{addr("1"), addr("4"), addr("3"), addr("2")}; !slices.Equal(order, want) {
		t.Errorf("order %v, want %v", order, want)
	}
}

func TestStoppingAQueueSendsItsDownloadsBackInLine(t *testing.T) {
	te := queuesEngine(t)
	c := te.add(t, AddRequest{URL: addr("c"), Queue: "night"})
	te.StartQueue("night")
	te.waitRunning(t, c.ID)

	te.StopQueue("night")
	te.waitQueueEvent(t, QueueEvent{QueueStopped, "night"})
	te.waitFor(t, c.ID, StatusQueued)
	te.staysQueued(t, c)

	te.StartQueue("night")
	jobs := te.runner.waitJobs(t, 2)
	if jobs[1].WorkDir != jobs[0].WorkDir {
		t.Error("it started over instead of carrying on")
	}
	te.waitFor(t, c.ID, StatusDownloading)
}

func TestFewerAtOnceStopsTheLastInLine(t *testing.T) {
	te := queuesEngine(t)
	c := te.add(t, AddRequest{URL: addr("c"), Queue: "night"})
	d := te.add(t, AddRequest{URL: addr("d"), Queue: "night"})
	te.StartQueue("night")
	te.waitRunning(t, c.ID)
	te.waitRunning(t, d.ID)

	settings := te.settings // what it started with
	settings.Queues = []QueueSettings{{ID: MainQueue, AtOnce: 1}, {ID: "night", AtOnce: 1}}
	te.SetSettings(settings)
	te.waitFor(t, d.ID, StatusQueued)
	if s := te.statusOf(t, c.ID); s != StatusDownloading {
		t.Errorf("the first in line is %s", s)
	}
	te.staysQueued(t, d)

	te.runner.finish(c.URL, nil)
	te.waitFor(t, d.ID, StatusDownloading) // its turn again

	// The same limit again stops nothing, so one started by hand beyond it keeps running.
	e := te.add(t, AddRequest{URL: addr("e"), Queue: "night"})
	if err := te.StartNow(e.ID); err != nil {
		t.Fatal(err)
	}
	te.waitRunning(t, e.ID)
	te.SetSettings(settings)
	time.Sleep(30 * time.Millisecond)
	if s := te.statusOf(t, e.ID); s != StatusDownloading {
		t.Errorf("the one started by hand is %s", s)
	}
}

// A started queue with nothing left is done and stops; Main is done but keeps running.
func TestAQueueIsDoneWhenEverythingInItIs(t *testing.T) {
	te := queuesEngine(t)
	c := te.add(t, AddRequest{URL: addr("c"), Queue: "night"})
	d := te.add(t, AddRequest{URL: addr("d"), Queue: "night"})
	paused := te.add(t, AddRequest{URL: addr("p"), Queue: "night", Paused: true})
	te.StartQueue("night")
	te.waitRunning(t, c.ID)
	te.runner.finish(c.URL, nil)
	te.runner.finish(d.URL, ErrBadURL) // a failure is done too
	te.waitQueueEvent(t, QueueEvent{QueueDone, "night"})
	if te.QueueRunning("night") {
		t.Error("Tonight runs on with nothing to do")
	}
	if s := te.statusOf(t, paused.ID); s != StatusPaused {
		t.Errorf("a paused download in it became %s", s)
	}

	a := te.add(t, AddRequest{URL: addr("a")})
	te.waitRunning(t, a.ID)
	te.runner.finish(a.URL, nil)
	te.waitQueueEvent(t, QueueEvent{QueueDone, MainQueue})
	if !te.QueueRunning(MainQueue) {
		t.Error("Main stopped")
	}
	b := te.add(t, AddRequest{URL: addr("b")})
	te.waitFor(t, b.ID, StatusDownloading)
}

// A queue started with nothing in it isn't done straight away: what is added to it runs.
func TestAnEmptyQueueWaitsForDownloads(t *testing.T) {
	te := queuesEngine(t)
	te.StartQueue("night")
	time.Sleep(30 * time.Millisecond)
	if !te.QueueRunning("night") || slices.Contains(te.queueHeard(), QueueEvent{QueueDone, "night"}) {
		t.Fatalf("an empty queue was done at once: %+v", te.queueHeard())
	}
	c := te.add(t, AddRequest{URL: addr("c"), Queue: "night"})
	te.waitFor(t, c.ID, StatusDownloading)
}

func TestMovingADownloadToAnotherQueue(t *testing.T) {
	te := queuesEngine(t)
	a := te.add(t, AddRequest{URL: addr("a")})
	b := te.add(t, AddRequest{URL: addr("b")})
	te.waitRunning(t, a.ID)

	// The running one moves to Tonight, which doesn't run: it waits there, and Main's next starts.
	if err := te.MoveToQueue([]string{a.ID}, "night"); err != nil {
		t.Fatal(err)
	}
	te.waitFor(t, a.ID, StatusQueued)
	te.waitFor(t, b.ID, StatusDownloading)
	if d, _ := te.Get(a.ID); d.Queue != "night" {
		t.Errorf("queue = %q", d.Queue)
	}

	// Back to Main, behind b.
	if err := te.MoveToQueue([]string{a.ID}, MainQueue); err != nil {
		t.Fatal(err)
	}
	te.staysQueued(t, a)
	te.runner.finish(b.URL, nil)
	te.waitFor(t, a.ID, StatusDownloading)
}

func TestStartNowJumpsTheQueue(t *testing.T) {
	te := queuesEngine(t)
	c := te.add(t, AddRequest{URL: addr("c"), Queue: "night"})
	a := te.add(t, AddRequest{URL: addr("a")})
	b := te.add(t, AddRequest{URL: addr("b")})
	te.waitRunning(t, a.ID)
	for _, d := range []Download{c, b} {
		if err := te.StartNow(d.ID); err != nil {
			t.Fatal(err)
		}
		te.waitFor(t, d.ID, StatusDownloading)
	}
}

func TestConfirmPutsItInAQueue(t *testing.T) {
	te := newTestEngine(t, Settings{Ask: true, Queues: []QueueSettings{{ID: "night", AtOnce: 1}}})
	d := te.add(t, AddRequest{URL: addr("asked")})
	if err := te.Confirm(d.ID, ConfirmRequest{Queue: "night"}); err != nil {
		t.Fatal(err)
	}
	te.staysQueued(t, d)
	if got, _ := te.Get(d.ID); got.Queue != "night" || got.Position <= d.Position {
		t.Errorf("confirmed into %q at %d (added at %d)", got.Queue, got.Position, d.Position)
	}
}

func TestADeletedQueuesDownloadsRunInMain(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: addr("orphan"), Queue: "gone"})
	te.waitFor(t, d.ID, StatusDownloading)
}

func TestAnOldListJoinsMainOldestFirst(t *testing.T) {
	te := newTestEngine(t, Settings{MaxActive: 1})
	te.Shutdown(t.Context())
	old := `[
		{"id": "new", "url": "https://example.com/new", "kind": "http", "status": "queued", "directory": "` + te.dir + `", "created": "2026-10-02T00:00:00Z", "work": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"id": "old", "url": "https://example.com/old", "kind": "http", "status": "queued", "directory": "` + te.dir + `", "created": "2026-10-01T00:00:00Z", "work": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	]`
	path := filepath.Join(filepath.Dir(te.workRoot), "state", "downloads.json")
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	te.start(t)
	te.waitFor(t, "old", StatusDownloading)
	if d, _ := te.Get("new"); d.Queue != MainQueue || d.Status != StatusQueued {
		t.Errorf("the newer one: %+v", d)
	}
	added := te.add(t, AddRequest{URL: addr("later")})
	if newer, _ := te.Get("new"); added.Position <= newer.Position {
		t.Errorf("a new download got place %d, ahead of the old list's %d", added.Position, newer.Position)
	}
}

// A queue's speed limit is shared by its downloads, and changes for them while they run.
func TestAQueuesDownloadsShareItsSpeedLimit(t *testing.T) {
	settings := Settings{MaxActive: 3, Queues: []QueueSettings{{ID: MainQueue, AtOnce: 2, SpeedLimit: 1000}}}
	te := newTestEngine(t, settings)
	a := te.add(t, AddRequest{URL: addr("a")})
	b := te.add(t, AddRequest{URL: addr("b")})
	te.waitRunning(t, a.ID)
	te.waitRunning(t, b.ID)
	jobs := te.runner.waitJobs(t, 2)
	if jobs[0].Limit == nil || jobs[0].Limit != jobs[1].Limit || jobs[0].Limit.Rate() != 1000 {
		t.Fatalf("limits %p and %p, want one shared at 1000", jobs[0].Limit, jobs[1].Limit)
	}
	if jobs[0].SpeedLimit != 500 {
		t.Errorf("each one's share = %d, want 500", jobs[0].SpeedLimit)
	}
	settings.Queues[0].SpeedLimit = 0
	te.SetSettings(settings)
	if jobs[0].Limit.Rate() != 0 {
		t.Error("lifting the limit didn't reach the running downloads")
	}

	te2 := newTestEngine(t, Settings{})
	c := te2.add(t, AddRequest{URL: addr("c")})
	te2.waitRunning(t, c.ID)
	if job := te2.runner.waitJobs(t, 1)[0]; job.Limit != nil || job.SpeedLimit != 0 {
		t.Errorf("a queue without a limit limited its download: %+v", job)
	}
}

func TestAFailedDownloadIsRetriedBeforeTheNext(t *testing.T) {
	te := newTestEngine(t, Settings{MaxActive: 1, Queues: []QueueSettings{{
		ID: MainQueue, AtOnce: 1, Retries: 2, RetryWait: 100 * time.Millisecond,
	}}})
	a := te.add(t, AddRequest{URL: addr("a.bin")})
	b := te.add(t, AddRequest{URL: addr("b.bin")})
	te.waitFor(t, a.ID, StatusDownloading)

	te.runner.finish(a.URL, errNope)
	first := waitNote(t, te, a.ID, "1 of 2")
	if first.Status != StatusDownloading {
		t.Fatalf("while waiting to retry: %s", first.Status)
	}
	if s := te.statusOf(t, b.ID); s != StatusQueued {
		t.Fatalf("the next one started while the first is being tried again: %s", s)
	}

	te.runner.waitJobs(t, 2)
	te.runner.finish(a.URL, errNope)
	waitNote(t, te, a.ID, "2 of 2")
	if s := te.statusOf(t, b.ID); s != StatusQueued {
		t.Fatalf("the next one started on the second try: %s", s)
	}

	te.runner.waitJobs(t, 3)
	te.runner.finish(a.URL, errNope)
	failed := te.waitFor(t, a.ID, StatusError)
	if failed.Error != errNope.Error() {
		t.Errorf("error = %q", failed.Error)
	}
	te.waitFor(t, b.ID, StatusDownloading)
}

func TestPausingARetryLetsTheNextStart(t *testing.T) {
	te := newTestEngine(t, Settings{MaxActive: 1, Queues: []QueueSettings{{
		ID: MainQueue, AtOnce: 1, Retries: 3, RetryWait: time.Hour,
	}}})
	a := te.add(t, AddRequest{URL: addr("a.bin")})
	b := te.add(t, AddRequest{URL: addr("b.bin")})
	te.waitFor(t, a.ID, StatusDownloading)
	te.runner.finish(a.URL, errNope)
	waitNote(t, te, a.ID, "1 of 3")

	if err := te.Pause(a.ID); err != nil {
		t.Fatal(err)
	}
	if d, _ := te.Get(a.ID); d.Status != StatusPaused {
		t.Fatalf("paused = %s", d.Status)
	}
	te.waitFor(t, b.ID, StatusDownloading)
}

func TestNoMatchAsksInsteadOfRetrying(t *testing.T) {
	te := newTestEngine(t, Settings{MaxActive: 1, Queues: []QueueSettings{{
		ID: MainQueue, AtOnce: 1, Retries: 3, RetryWait: time.Hour,
	}}})
	a := te.add(t, AddRequest{URL: addr("a.bin")})
	b := te.add(t, AddRequest{URL: addr("b.bin")})
	te.waitFor(t, a.ID, StatusDownloading)
	te.runner.finish(a.URL, &NeedsPlace{Text: "couldn't find “Bitab” on YouTube Music: no match"})

	d := te.waitFor(t, a.ID, StatusError)
	if !d.Where || !strings.Contains(d.Error, "Bitab") {
		t.Fatalf("waiting %+v", d)
	}
	te.waitFor(t, b.ID, StatusDownloading)
	if err := te.UseFound(a.ID, "not a link"); err == nil {
		t.Fatal("a name was taken as a link")
	}
	if err := te.UseFound(a.ID, "https://www.youtube.com/watch?v=bitab"); err != nil {
		t.Fatal(err)
	}
	got, err := te.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusQueued || got.Where || got.Resolve || got.Error != "" ||
		got.URL != "https://www.youtube.com/watch?v=bitab" || got.Page != a.URL {
		t.Fatalf("pointed at %+v", got)
	}
}

func waitNote(t *testing.T, te *testEngine, id, part string) Download {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		d, err := te.Get(id)
		if err == nil && strings.Contains(d.Note, part) {
			return d
		}
		time.Sleep(5 * time.Millisecond)
	}
	d, _ := te.Get(id)
	t.Fatalf("note of %s = %q, want %q", id, d.Note, part)
	return d
}
