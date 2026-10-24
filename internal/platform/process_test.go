package platform

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

// The test binary re-runs itself as a parent with a child, like yt-dlp and ffmpeg. Go programs
// are used because a shell ignores Ctrl-C in its background jobs.
func init() {
	switch os.Getenv("SNAG_TEST_GROUP") {
	case "parent":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), "SNAG_TEST_GROUP=child")
		child.Stdout = os.Stdout
		if err := child.Start(); err != nil {
			os.Exit(1)
		}
		fmt.Println("ready")
		_ = child.Wait()
		os.Exit(0)
	case "child":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func TestStopGroupEndsTheProgramAndItsChildren(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "SNAG_TEST_GROUP=parent")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	StartGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewReader(out)
	if line, err := lines.ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("the program said %q, %v", line, err)
	}

	if err := StopGroup(cmd, false); err != nil {
		t.Fatal(err)
	}
	// The child shares the output, so it closes only when both have ended.
	closed := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, lines)
		close(closed)
	}()
	select {
	case <-closed:
		_ = cmd.Wait()
	case <-time.After(10 * time.Second):
		_ = StopGroup(cmd, true)
		t.Fatal("still running after StopGroup")
	}
}
