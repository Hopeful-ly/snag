package streamdl

import (
	"os"
)

// playStart is how many bytes of picture a player needs before it starts.
const playStart = 1 << 20

// soundStart is how many bytes of separate sound a player needs alongside.
const soundStart = 256 << 10

func (s *stream) offer() {
	s.mu.Lock()
	offer := !s.readable
	s.readable = true
	s.mu.Unlock()
	if offer {
		s.report.Readable(s)
	}
}

func (s *stream) setPicture(path string) {
	s.mu.Lock()
	s.picture = path
	s.mu.Unlock()
	s.offer()
}

// Path is the picture's (or single-file video's) current file, including a .part suffix.
func (s *stream) Path() string {
	s.mu.Lock()
	name := s.picture
	if len(s.parts) > 0 {
		name = s.parts[0].file
	}
	s.mu.Unlock()
	return current(name)
}

// Have returns how much of the picture there is from off; 0 until a separate sound has its start.
func (s *stream) Have(off int64) int64 {
	if s.soundLeft() > 0 {
		return 0
	}
	path := s.Path()
	if path == "" {
		return 0
	}
	return max(fileSize(path)-off, 0)
}

// Want does nothing: yt-dlp fetches a stream in order.
func (s *stream) Want(int64) {}

// Sound is the separate sound's current file ("" until it begins); false when there is none.
func (s *stream) Sound() (string, bool) {
	s.mu.Lock()
	if len(s.parts) == 0 {
		s.mu.Unlock()
		return "", false
	}
	name := s.parts[1].file
	s.mu.Unlock()
	return current(name), true
}

func (s *stream) soundLeft() int64 {
	s.mu.Lock()
	if len(s.parts) == 0 {
		s.mu.Unlock()
		return 0
	}
	name, size, finished := s.parts[1].file, s.parts[1].size, s.parts[1].finished
	s.mu.Unlock()
	if finished {
		return 0
	}
	need := int64(soundStart)
	if size > 0 {
		need = min(need, size)
	}
	have := int64(0)
	if name != "" {
		have = fileSize(name + ".part")
	}
	return max(need-have, 0)
}

// toPlay is how many bytes must still arrive before the video can be watched.
func (s *stream) toPlay() int64 {
	sound := s.soundLeft()
	picture := int64(0)
	if path := s.Path(); path != "" {
		picture = fileSize(path)
	}
	return max(playStart-picture, 0) + sound
}

// current is name+".part" while yt-dlp writes it, else name if it exists, else "".
func current(name string) string {
	switch {
	case name == "":
		return ""
	case exists(name + ".part"):
		return name + ".part"
	case exists(name):
		return name
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
