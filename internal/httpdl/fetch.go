package httpdl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/speed"
)

const chunkSize = 64 << 10

var errIncomplete = errors.New("download incomplete")

func (d *download) fetch(ctx context.Context, first *http.Response) error {
	var wg sync.WaitGroup
	launch := func(seg *segment, resp *http.Response) {
		d.workers++
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.work(ctx, seg, resp)
		}()
	}

	d.mu.Lock()
	connections := 1
	if d.resumable && d.size > minSplit {
		connections = max(1, d.job.Connections)
	}
	d.limit = connections
	if d.resumable {
		d.launch = launch
	}
	if first != nil {
		seg := d.segs.list[0]
		seg.busy = true
		launch(seg, first)
	}
	for d.workers < connections {
		seg := d.segs.take(d.resumable)
		if seg == nil {
			break
		}
		launch(seg, nil)
	}
	d.mu.Unlock()

	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()
	d.watch(finished)
	d.mu.Lock()
	d.launch = nil
	d.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.segs.remaining() > 0 {
		if d.failure != nil {
			return d.failure
		}
		return errIncomplete
	}
	return nil
}

// watch reports progress and periodically saves resume state until finished is closed.
func (d *download) watch(finished <-chan struct{}) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	var meter speed.Meter
	lastSave := time.Now()
	for {
		select {
		case <-finished:
			return
		case now := <-ticker.C:
			d.mu.Lock()
			done := d.done
			d.mu.Unlock()
			d.progress(meter.Sample(done, now))
			if now.Sub(lastSave) >= stateInterval {
				lastSave = now
				d.saveState()
			}
		}
	}
}

// work is one connection, filling segments until none are left or it gives up.
func (d *download) work(ctx context.Context, seg *segment, resp *http.Response) {
	for seg != nil && ctx.Err() == nil {
		complete := d.fill(ctx, seg, resp)
		resp = nil
		d.mu.Lock()
		seg.busy = false
		seg = nil
		if complete && d.workers <= d.limit { // an extra connection stops after its part
			seg = d.segs.take(d.resumable)
		}
		d.mu.Unlock()
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if seg != nil {
		seg.busy = false
	}
	d.workers--
}

// fill downloads one segment, reconnecting after errors, and reports whether it completed; on
// giving up, d.failure says why.
func (d *download) fill(ctx context.Context, seg *segment, resp *http.Response) bool {
	for try := 1; ; try++ {
		err := d.fillOnce(ctx, seg, resp)
		resp = nil
		if err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		d.mu.Lock()
		resumable := d.resumable
		d.mu.Unlock()
		if isPermanent(err) || try > maxRetries || !resumable {
			d.fail(err)
			return false
		}
		wait := d.runner.backoff(try)
		d.report.Said(fmt.Sprintf("A connection failed (%v); trying it again in %v, %d of %d", err, wait.Round(time.Second), try, maxRetries))
		select {
		case <-ctx.Done():
			return false
		case <-time.After(wait):
		}
	}
}

func (d *download) fillOnce(ctx context.Context, seg *segment, resp *http.Response) error {
	if resp == nil {
		d.mu.Lock()
		from, to := seg.pos, seg.end
		if d.size == 0 {
			to = -1
		}
		d.mu.Unlock()

		var err error
		if resp, err = d.get(ctx, from, to); err != nil {
			return err
		}
		if resp.StatusCode != http.StatusPartialContent && from > 0 {
			resp.Body.Close()
			return errNoResume
		}
	}
	defer resp.Body.Close()

	buf := make([]byte, chunkSize)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 && d.job.Limit != nil {
			if err := d.job.Limit.Wait(ctx, n); err != nil {
				return err
			}
		}
		if n > 0 {
			full, writeErr := d.write(seg, buf[:n])
			if writeErr != nil {
				return writeErr
			}
			if full {
				return nil
			}
		}
		if errors.Is(err, io.EOF) {
			return d.ended(seg)
		}
		if err != nil {
			return err
		}
	}
}

func (d *download) write(seg *segment, data []byte) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	data = data[:min(int64(len(data)), seg.left())]
	if _, err := d.file.WriteAt(data, seg.pos); err != nil {
		return false, &diskError{err}
	}
	seg.pos += int64(len(data))
	d.done += int64(len(data))
	if !d.sniffed && seg.start == 0 && seg.pos >= sniffSize {
		d.sniffed = true
		d.segs.tailFrom = d.mp4Tail()
	}
	return seg.left() <= 0, nil
}

// ended handles a body that ran out: the end of an unknown-size file, or an early close.
func (d *download) ended(seg *segment) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.size == 0 {
		seg.end = seg.pos - 1
		d.size = seg.pos
		return nil
	}
	if seg.left() > 0 {
		return errClosedEarly
	}
	return nil
}

// fail records only the first reason a connection gave up.
func (d *download) fail(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failure == nil {
		d.failure = err
	}
}
