package platform

// Open opens a file with its usual program.
func Open(path string) error {
	return launch("open", path)
}

// Reveal shows a file in Finder, selected.
func Reveal(path string) error {
	return launch("open", "-R", path)
}
