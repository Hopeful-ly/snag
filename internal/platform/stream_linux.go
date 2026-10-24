package platform

import (
	"bufio"
	"cmp"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// PlayStream opens address in the default player for files like name, if its Exec takes %u or %U.
func PlayStream(address, name string) error {
	id := defaultPlayer(name)
	if id == "" {
		return ErrNoStreamPlayer
	}
	return PlayStreamWith(id, address)
}

// PlayStreamWith opens address in player id from StreamPlayers.
func PlayStreamWith(id, address string) error {
	entry := findDesktopEntry(id)
	if entry == "" {
		return ErrNoStreamPlayer
	}
	launcher, err := readDesktopEntry(entry)
	if err != nil {
		return ErrNoStreamPlayer
	}
	args, ok := streamArgs(launcher.exec, address)
	if !ok {
		return ErrNoStreamPlayer
	}
	return launch(args[0], args[1:]...)
}

// StreamPlayers lists installed players that take stream addresses, the default for name first.
func StreamPlayers(name string) []Player {
	home, _ := os.UserHomeDir()
	def := defaultPlayer(name)
	seen := map[string]bool{}
	var players []Player
	for _, dir := range applicationDirs(home) {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			id := e.Name()
			if !strings.HasSuffix(id, ".desktop") || seen[id] {
				continue
			}
			seen[id] = true // the first folder's entry hides the others'
			launcher, err := readDesktopEntry(filepath.Join(dir, id))
			if err != nil || !launcher.playsStreams() {
				continue
			}
			players = append(players, Player{ID: id, Name: launcher.name, Default: id == def})
		}
	}
	slices.SortStableFunc(players, func(a, b Player) int {
		if a.Default != b.Default {
			if a.Default {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return players
}

// defaultApp returns the default launcher id for a MIME type, or "". Tests replace it.
var defaultApp = xdgDefault

func xdgDefault(mimeType string) string {
	out, err := exec.Command("xdg-mime", "query", "default", mimeType).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// defaultPlayer returns the default player for files like name if it plays streams, else "".
func defaultPlayer(name string) string {
	kind, _, _ := strings.Cut(mime.TypeByExtension(filepath.Ext(name)), ";")
	id := defaultApp(cmp.Or(kind, "video/mp4"))
	entry := findDesktopEntry(id)
	if entry == "" {
		return ""
	}
	if launcher, err := readDesktopEntry(entry); err != nil || !launcher.takesAddresses() {
		return ""
	}
	return id
}

// findDesktopEntry returns the path of an application's .desktop file, where the desktop looks.
func findDesktopEntry(id string) string {
	if id == "" || strings.ContainsAny(id, "/\\") {
		return ""
	}
	home, _ := os.UserHomeDir()
	for _, dir := range applicationDirs(home) {
		path := filepath.Join(dir, id)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// applicationDirs lists launcher folders, highest precedence first.
func applicationDirs(home string) []string {
	dirs := []string{cmp.Or(os.Getenv("XDG_DATA_HOME"), filepath.Join(home, ".local", "share"))}
	dirs = append(dirs, filepath.SplitList(cmp.Or(os.Getenv("XDG_DATA_DIRS"), "/usr/local/share:/usr/share"))...)
	for i, dir := range dirs {
		dirs[i] = filepath.Join(dir, "applications")
	}
	return dirs
}

type launcher struct {
	name, exec, mimeTypes string
	hidden                bool
}

func (l launcher) takesAddresses() bool {
	_, ok := streamArgs(l.exec, "http://x")
	return ok
}

// playsStreams excludes browsers, which open video too but aren't players.
func (l launcher) playsStreams() bool {
	return !l.hidden && strings.Contains(l.mimeTypes, "video/") && l.takesAddresses() &&
		!strings.Contains(l.mimeTypes, "text/html") && !strings.Contains(l.mimeTypes, "x-scheme-handler/http")
}

func readDesktopEntry(path string) (launcher, error) {
	f, err := os.Open(path)
	if err != nil {
		return launcher{}, err
	}
	defer f.Close()
	var l launcher
	main := false
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(line, "[") {
			main = line == "[Desktop Entry]"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !main || !ok {
			continue
		}
		switch key {
		case "Name":
			l.name = value
		case "Exec":
			l.exec = value
		case "MimeType":
			l.mimeTypes = value
		case "NoDisplay", "Hidden":
			l.hidden = l.hidden || value == "true"
		}
	}
	if l.exec == "" {
		return l, os.ErrNotExist
	}
	return l, lines.Err()
}

// streamArgs expands an Exec line for address per the Desktop Entry spec: %u/%U take it, other
// field codes are dropped. ok is false when the program takes only files.
func streamArgs(exec, address string) (args []string, ok bool) {
	for _, word := range splitExec(exec) {
		switch {
		case word == "%u" || word == "%U":
			args, ok = append(args, address), true
		case strings.Contains(strings.ReplaceAll(word, "%%", ""), "%"):
			// files, the icon, the name, the launcher's path: nothing a stream fills in
		default:
			args = append(args, strings.ReplaceAll(word, "%%", "%"))
		}
	}
	return args, ok && len(args) > 1
}

// splitExec splits an Exec line into words, honouring double quotes and their escapes.
func splitExec(line string) []string {
	var words []string
	var word strings.Builder
	quoted, inWord := false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quoted && c == '\\' && i+1 < len(line):
			i++
			word.WriteByte(line[i])
		case c == '"':
			quoted, inWord = !quoted, true
		case !quoted && (c == ' ' || c == '\t'):
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words
}
