package tools

// Apps started from the Finder get a PATH without Homebrew's folders, where ffmpeg and node
// usually are on a Mac.
var wellKnownDirs = []string{"/opt/homebrew/bin", "/usr/local/bin"}
