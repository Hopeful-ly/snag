package checksum

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	sha256Hex = "c2c6c6c9e1e1f6f3c4ba8f0f5d8c2b8e3f0a1b2c3d4e5f60718293a4b5c6d7e8"
	sha512Hex = "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
	md5Hex    = "d41d8cd98f00b204e9800998ecf8427e"
	sha1Hex   = "da39a3ee5e6b4b0d3255bfef95601890afd80709"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		text string
		want Sum
	}{
		{sha256Hex, Sum{SHA256, sha256Hex}},
		{"  " + strings.ToUpper(sha256Hex) + "\n", Sum{SHA256, sha256Hex}},
		{"sha256:" + sha256Hex, Sum{SHA256, sha256Hex}},
		{"SHA-256: " + sha256Hex, Sum{SHA256, sha256Hex}},
		{"SHA256 " + sha256Hex, Sum{SHA256, sha256Hex}},
		{"sha512sum " + sha512Hex, Sum{SHA512, sha512Hex}},
		{"MD5: " + md5Hex, Sum{MD5, md5Hex}},
		{sha1Hex, Sum{SHA1, sha1Hex}},
		{sha256Hex + "  ubuntu.iso", Sum{SHA256, sha256Hex}},
		{"SHA256 (Fedora.iso) = " + sha256Hex, Sum{SHA256, sha256Hex}},
	} {
		got, ok := Parse(tc.text)
		if !ok || got != tc.want {
			t.Errorf("Parse(%q) = %v, %v; want %v", tc.text, got, ok, tc.want)
		}
	}
	for _, bad := range []string{
		"", "hello", sha256Hex[:63], sha256Hex + "0", "sha512:" + sha256Hex, // label doesn't match length
		"zz" + sha256Hex[2:],
	} {
		if got, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) = %v; want nothing", bad, got)
		}
	}
}

func TestFindInRealFiles(t *testing.T) {
	other := strings.Repeat("ab", 32)
	for _, tc := range []struct {
		name, file, sums string
		want             Sum
	}{
		{
			name: "Ubuntu (binary marks)",
			file: "ubuntu-24.04.1-desktop-amd64.iso",
			sums: other + " *ubuntu-24.04.1-live-server-amd64.iso\n" + sha256Hex + " *ubuntu-24.04.1-desktop-amd64.iso\n",
			want: Sum{SHA256, sha256Hex},
		},
		{
			name: "Arch",
			file: "archlinux-2024.10.01-x86_64.iso",
			sums: sha256Hex + "  archlinux-2024.10.01-x86_64.iso\n" + other + "  archlinux-bootstrap-2024.10.01-x86_64.tar.zst\n",
			want: Sum{SHA256, sha256Hex},
		},
		{
			name: "Fedora (signed, BSD lines)",
			file: "Fedora-Workstation-Live-x86_64-40-1.14.iso",
			sums: "-----BEGIN PGP SIGNED MESSAGE-----\nHash: SHA256\n\n# Fedora-Workstation-Live-x86_64-40-1.14.iso: 2295853056 bytes\n" +
				"SHA256 (Fedora-Workstation-Live-x86_64-40-1.14.iso) = " + sha256Hex + "\n-----BEGIN PGP SIGNATURE-----\n\niQIzBAEB\n-----END PGP SIGNATURE-----\n",
			want: Sum{SHA256, sha256Hex},
		},
		{
			name: "Debian SHA512SUMS",
			file: "debian-12.7.0-amd64-netinst.iso",
			sums: sha512Hex + "  debian-12.7.0-amd64-netinst.iso\n",
			want: Sum{SHA512, sha512Hex},
		},
		{
			name: "a file for one file, without its name",
			file: "tool.zip",
			sums: sha256Hex + "\n",
			want: Sum{SHA256, sha256Hex},
		},
		{
			name: "Windows line ends, a path, another case",
			file: "Setup.EXE",
			sums: other + "  dist/readme.txt\r\n" + strings.ToUpper(sha256Hex) + "  ./dist/setup.exe\r\n",
			want: Sum{SHA256, sha256Hex},
		},
		{
			name: "several kinds: the strongest",
			file: "a.tar.gz",
			sums: md5Hex + "  a.tar.gz\n" + "SHA512 (a.tar.gz) = " + sha512Hex + "\n" + sha256Hex + "  a.tar.gz\n",
			want: Sum{SHA512, sha512Hex},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Find([]byte(tc.sums), tc.file)
			if !ok || got != tc.want {
				t.Fatalf("Find = %v, %v; want %v", got, ok, tc.want)
			}
		})
	}
}

func TestFindMissesOtherFiles(t *testing.T) {
	sums := sha256Hex + "  other.iso\n" + strings.Repeat("ab", 32) + "  another.iso\n"
	if got, ok := Find([]byte(sums), "mine.iso"); ok {
		t.Fatalf("Find = %v; want nothing", got)
	}
	if got, ok := Find([]byte("<html><body>Not found</body></html>"), "mine.iso"); ok {
		t.Fatalf("Find in a page = %v; want nothing", got)
	}
	// Two unnamed checksums say nothing about which is whose.
	if got, ok := Find([]byte(sha256Hex+"\n"+strings.Repeat("ab", 32)+"\n"), "mine.iso"); ok {
		t.Fatalf("Find = %v; want nothing", got)
	}
}

func b64(hexText string) string {
	raw, _ := hex.DecodeString(hexText)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestFromHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header http.Header
		want   Sum
	}{
		{"Repr-Digest", http.Header{"Repr-Digest": {"sha-256=:" + b64(sha256Hex) + ":"}}, Sum{SHA256, sha256Hex}},
		{"Digest, the strongest", http.Header{"Digest": {"md5=" + b64(md5Hex) + ", SHA-512=" + b64(sha512Hex)}}, Sum{SHA512, sha512Hex}},
		{"Google Cloud Storage", http.Header{"X-Goog-Hash": {"crc32c=n03x6A==", "md5=" + b64(md5Hex)}}, Sum{MD5, md5Hex}},
		{"Content-MD5", http.Header{"Content-Md5": {b64(md5Hex)}}, Sum{MD5, md5Hex}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := FromHeaders(tc.header)
			if !ok || got != tc.want {
				t.Fatalf("FromHeaders = %v, %v; want %v", got, ok, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		header http.Header
	}{
		{"none", http.Header{}},
		{"the part's, not the file's", http.Header{"Content-Digest": {"sha-256=:" + b64(sha256Hex) + ":"}}},
		{"compressed", http.Header{"Content-Encoding": {"gzip"}, "Content-Md5": {b64(md5Hex)}}},
		{"unknown", http.Header{"Digest": {"unixsum=30637"}}},
		{"broken", http.Header{"Repr-Digest": {"sha-256=:nope:"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := FromHeaders(tc.header); ok {
				t.Fatalf("FromHeaders = %v; want nothing", got)
			}
		})
	}
}

func TestFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.bin")
	data := []byte(strings.Repeat("snag", 700_000)) // more than one buffer
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	var last int64
	got, err := File(context.Background(), path, SHA256, func(n int64) { last = n })
	if err != nil {
		t.Fatal(err)
	}
	if got != hex.EncodeToString(sum[:]) {
		t.Fatalf("File = %s, want %x", got, sum)
	}
	if last != int64(len(data)) {
		t.Fatalf("progress ended at %d, want %d", last, len(data))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := File(ctx, path, SHA256, nil); err == nil {
		t.Fatal("File went on after it was canceled")
	}
	if _, err := File(context.Background(), path, "crc", nil); err == nil {
		t.Fatal("File knew an unknown algorithm")
	}
}

func TestNames(t *testing.T) {
	if SHA256.Name() != "SHA-256" || MD5.Name() != "MD5" || !SHA1.Weak() || SHA512.Weak() {
		t.Fatal("names or weakness wrong")
	}
}
