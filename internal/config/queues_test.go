package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Settings from before queues get a Main queue that runs as many at once as they did.
func TestMainQueueComesFromMaxActive(t *testing.T) {
	c := Default("/downloads")
	c.MaxActive = 5
	c = c.Normalized()
	if len(c.Queues) != 1 || c.Queues[0].ID != MainQueue || c.Queues[0].Name != "Main" || c.Queues[0].AtOnce != 5 {
		t.Fatalf("queues = %+v", c.Queues)
	}
	main := c.Queues[0]
	main.AtOnce = 2
	main.WhenDone = WhenDoneShutdown
	c.SetQueue(main)
	c = c.Normalized()
	if c.MaxActive != 2 || c.Queues[0].AtOnce != 2 || c.Queues[0].WhenDone != WhenDoneShutdown {
		t.Errorf("Main's at once and MaxActive part ways: %d, %+v", c.MaxActive, c.Queues[0])
	}
	c.RemoveQueue(MainQueue)
	if _, ok := c.Queue(MainQueue); !ok {
		t.Error("Main was removed")
	}
}

func TestQueuesAreKeptWithinLimits(t *testing.T) {
	c := Default("/downloads")
	c.Queues = []Queue{
		{ID: "night", Name: "  Tonight ", AtOnce: 99, SpeedLimit: -5, WhenDone: "explode",
			Schedule: Schedule{On: true, Start: "02:00", Stop: "07:00", Days: []int{6, 1, 1, 9, 0}}},
		{ID: "night", Name: "Twice"},
		{ID: "", Name: "No id"},
		{ID: "broken", Schedule: Schedule{On: true, Start: "25:99", Stop: "nope", Days: []int{0, 1, 2, 3, 4, 5, 6}}},
		{ID: MainQueue, Name: "Main", WhenDone: WhenDoneSleep},
	}
	c = c.Normalized()
	ids := []string{}
	for _, q := range c.Queues {
		ids = append(ids, q.ID)
	}
	if !slices.Equal(ids, []string{MainQueue, "night", "broken"}) {
		t.Fatalf("queues %v, want Main first and no repeats or blank ids", ids)
	}
	if c.Queues[0].WhenDone != WhenDoneSleep {
		t.Errorf("Main's own settings were lost: %+v", c.Queues[0])
	}
	night := c.Queues[1]
	if night.Name != "Tonight" || night.AtOnce != maxActive || night.SpeedLimit != 0 || night.WhenDone != WhenDoneNothing {
		t.Errorf("night = %+v", night)
	}
	if s := night.Schedule; !s.On || s.Start != "02:00" || s.Stop != "07:00" || !slices.Equal(s.Days, []int{0, 1, 6}) {
		t.Errorf("night's schedule = %+v", s)
	}
	if s := c.Queues[2].Schedule; s.On || s.Start != "" || s.Stop != "" || s.Days != nil || c.Queues[2].Name != "Queue" {
		t.Errorf("a broken schedule should be off: %+v", c.Queues[2])
	}
}

func TestQueuesAreSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	c := Default("/downloads")
	c.SetQueue(Queue{ID: "night", Name: "Tonight", AtOnce: 2, SpeedLimit: 500 << 10,
		Schedule: Schedule{On: true, Start: "02:00"}})
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, Default("/downloads"))
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := got.Queue("night"); !ok || q.Name != "Tonight" || q.SpeedLimit != 500<<10 || q.Schedule.Start != "02:00" {
		t.Errorf("loaded %+v", got.Queues)
	}
}

func TestAQueueSavedBeforeRetriesTriesAgainByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	raw := `{
		"downloadDir": "/downloads",
		"queues": [{"id": "main", "name": "Main", "atOnce": 1, "schedule": {"on": false, "start": ""}}]
	}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, Default("/downloads"))
	if err != nil {
		t.Fatal(err)
	}
	main, ok := got.Queue(MainQueue)
	if !ok || main.Retries != DefaultRetries || main.RetryWait != DefaultRetryWait {
		t.Fatalf("Main = %+v, want %d tries, %d seconds apart", main, DefaultRetries, DefaultRetryWait)
	}

	main.Retries, main.RetryWait = 0, 0
	got.SetQueue(main)
	if err := Save(path, got); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path, Default("/downloads"))
	if err != nil {
		t.Fatal(err)
	}
	main, _ = again.Queue(MainQueue)
	if main.Retries != 0 || main.RetryWait != 0 {
		t.Fatalf("an explicit 0 was replaced: %+v", main)
	}
}

func TestClockTime(t *testing.T) {
	for in, want := range map[string]int{"00:00": 0, "02:30": 150, "23:59": 1439} {
		if got, ok := ClockTime(in); !ok || got != want {
			t.Errorf("ClockTime(%q) = %d, %v", in, got, ok)
		}
	}
	for _, bad := range []string{"", "24:00", "2:30pm", "noon"} {
		if _, ok := ClockTime(bad); ok {
			t.Errorf("ClockTime(%q) was read", bad)
		}
	}
}

func TestArrangeQueues(t *testing.T) {
	c := Config{Queues: []Queue{{ID: MainQueue}, {ID: "a"}, {ID: "b"}, {ID: "c"}}}
	c.ArrangeQueues([]string{"c", MainQueue, "a", "c"})
	var got []string
	for _, q := range c.Queues {
		got = append(got, q.ID)
	}
	if want := []string{MainQueue, "c", "a", "b"}; !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
}

func TestAQueueRemembersWhereToLook(t *testing.T) {
	c := Config{MaxActive: 1, Queues: []Queue{{ID: MainQueue, Name: "Main", Fallback: "bandcamp"}}}
	if got := c.Normalized().Queues[0].Fallback; got != "" {
		t.Errorf("fallback %q", got)
	}
	c.Queues[0].Fallback = "soundcloud"
	if got := c.Normalized().Queues[0].Fallback; got != "soundcloud" {
		t.Errorf("fallback %q", got)
	}
}
