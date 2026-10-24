package platform

// Open opens a file with its usual program.
func Open(path string) error {
	return launch("rundll32", "url.dll,FileProtocolHandler", path)
}

// Reveal shows a file in Explorer, selected.
func Reveal(path string) error {
	return launch("explorer", "/select,", path)
}
