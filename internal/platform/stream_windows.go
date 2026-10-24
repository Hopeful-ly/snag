package platform

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// knownPlayers accept a stream address on the command line; Windows' built-in players don't.
var knownPlayers = map[string]string{
	"vlc.exe": "VLC", "mpc-hc64.exe": "MPC-HC", "mpc-hc.exe": "MPC-HC", "mpc-be64.exe": "MPC-BE",
	"mpc-be.exe": "MPC-BE", "potplayermini64.exe": "PotPlayer", "potplayermini.exe": "PotPlayer",
	"potplayer64.exe": "PotPlayer", "potplayer.exe": "PotPlayer", "mpv.exe": "mpv", "mpvnet.exe": "mpv.net",
	"smplayer.exe": "SMPlayer",
}

// usualPlaces are checked when App Paths doesn't list a player.
var usualPlaces = []string{
	`VideoLAN\VLC\vlc.exe`, `MPC-HC\mpc-hc64.exe`, `MPC-HC\mpc-hc.exe`, `MPC-BE x64\mpc-be64.exe`,
	`MPC-BE\mpc-be.exe`, `DAUM\PotPlayer\PotPlayerMini64.exe`, `DAUM\PotPlayer\PotPlayerMini.exe`,
	`SMPlayer\smplayer.exe`, `mpv\mpv.exe`,
}

var assocQueryString = syscall.NewLazyDLL("shlwapi.dll").NewProc("AssocQueryStringW")

// PlayStream opens address in the default player for files like name, if it takes addresses.
func PlayStream(address, name string) error {
	exe := defaultProgram(extensionOf(name))
	if !playsStreams(exe) {
		return ErrNoStreamPlayer
	}
	return launch(exe, address)
}

// PlayStreamWith opens address in player id (a program path from StreamPlayers).
func PlayStreamWith(id, address string) error {
	if !playsStreams(id) {
		return ErrNoStreamPlayer
	}
	return launch(id, address)
}

// StreamPlayers lists installed players that take stream addresses, the default for name first.
func StreamPlayers(name string) []Player {
	def := strings.ToLower(defaultProgram(extensionOf(name)))
	found := map[string]string{} // by lower-cased path
	add := func(path string) {
		if path != "" && playsStreams(path) {
			if _, err := os.Stat(path); err == nil {
				found[strings.ToLower(path)] = path
			}
		}
	}
	add(def)
	for exe := range knownPlayers {
		for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
			add(appPath(root, exe))
		}
	}
	for _, dir := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA") + `\Programs`} {
		for _, place := range usualPlaces {
			if dir != "" {
				add(filepath.Join(dir, place))
			}
		}
	}
	var players []Player
	seen := map[string]bool{}
	for key, path := range found {
		name := knownPlayers[strings.ToLower(filepath.Base(path))]
		if seen[name] && key != def {
			continue // the same player twice (32- and 64-bit): one is enough
		}
		seen[name] = true
		players = append(players, Player{ID: path, Name: name, Default: key == def})
	}
	slices.SortFunc(players, func(a, b Player) int {
		if a.Default != b.Default {
			if a.Default {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return players
}

func playsStreams(exe string) bool {
	_, ok := knownPlayers[strings.ToLower(filepath.Base(exe))]
	return exe != "" && ok
}

func extensionOf(name string) string {
	if ext := strings.ToLower(filepath.Ext(name)); ext != "" {
		return ext
	}
	return ".mp4" // a stream named after its page's title
}

// appPath returns exe's path from the App Paths registry key, or "".
func appPath(root registry.Key, exe string) string {
	k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\App Paths\`+exe, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	path, _, err := k.GetStringValue("")
	if err != nil {
		return ""
	}
	return strings.Trim(path, `"`)
}

// defaultProgram returns the program that opens files with extension ext, or "".
func defaultProgram(ext string) string {
	extension, err := syscall.UTF16PtrFromString(ext)
	if err != nil {
		return ""
	}
	const executable = 2 // ASSOCSTR_EXECUTABLE
	buf := make([]uint16, syscall.MAX_PATH*2)
	size := uint32(len(buf))
	r, _, _ := assocQueryString.Call(0, executable, uintptr(unsafe.Pointer(extension)), 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 { // an HRESULT: 0 is success
		return ""
	}
	return syscall.UTF16ToString(buf)
}
