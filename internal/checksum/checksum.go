// Package checksum parses published checksums and checks files against them.
package checksum

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"  //nolint:gosec // only to compare with what a site publishes
	"crypto/sha1" //nolint:gosec // the same
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

// Algo is a hash algorithm.
type Algo string

// Supported algorithms, weakest first.
const (
	MD5    Algo = "md5"
	SHA1   Algo = "sha1"
	SHA256 Algo = "sha256"
	SHA384 Algo = "sha384"
	SHA512 Algo = "sha512"
)

// Name is the display name, such as "SHA-256".
func (a Algo) Name() string {
	switch a {
	case MD5:
		return "MD5"
	case SHA1:
		return "SHA-1"
	case SHA256:
		return "SHA-256"
	case SHA384:
		return "SHA-384"
	case SHA512:
		return "SHA-512"
	}
	return strings.ToUpper(string(a))
}

// Weak reports whether a is broken against deliberate collisions, so a match only rules out damage.
func (a Algo) Weak() bool { return a == MD5 || a == SHA1 }

func (a Algo) new() hash.Hash {
	switch a {
	case MD5:
		return md5.New() //nolint:gosec // see the import
	case SHA1:
		return sha1.New() //nolint:gosec // see the import
	case SHA256:
		return sha256.New()
	case SHA384:
		return sha512.New384()
	case SHA512:
		return sha512.New()
	}
	return nil
}

func (a Algo) strength() int {
	switch a {
	case MD5:
		return 1
	case SHA1:
		return 2
	case SHA256:
		return 3
	case SHA384:
		return 4
	case SHA512:
		return 5
	}
	return 0
}

func byLength(n int) (Algo, bool) {
	switch n {
	case 32:
		return MD5, true
	case 40:
		return SHA1, true
	case 64:
		return SHA256, true
	case 96:
		return SHA384, true
	case 128:
		return SHA512, true
	}
	return "", false
}

// byName accepts names such as "SHA256", "sha-256", "md5".
func byName(name string) (Algo, bool) {
	switch strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "-", "") {
	case "md5":
		return MD5, true
	case "sha1":
		return SHA1, true
	case "sha256":
		return SHA256, true
	case "sha384":
		return SHA384, true
	case "sha512":
		return SHA512, true
	}
	return "", false
}

// Sum is a checksum; Hex is lower-case.
type Sum struct {
	Algo Algo
	Hex  string
}

// IsZero reports whether s is empty.
func (s Sum) IsZero() bool { return s.Hex == "" }

// Better reports whether s is a stronger checksum than other.
func (s Sum) Better(other Sum) bool {
	return !s.IsZero() && (other.IsZero() || s.Algo.strength() > other.Algo.strength())
}

func fromHex(text string, named Algo) (Sum, bool) {
	text = strings.ToLower(text)
	algo, ok := byLength(len(text))
	if !ok || (named != "" && named != algo) {
		return Sum{}, false
	}
	if _, err := hex.DecodeString(text); err != nil {
		return Sum{}, false
	}
	return Sum{Algo: algo, Hex: text}, true
}

func fromBase64(text string, algo Algo) (Sum, bool) {
	text = strings.Trim(strings.TrimSpace(text), ":")
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return Sum{}, false
	}
	return fromHex(hex.EncodeToString(raw), algo)
}

var (
	// "SHA256 (name) = hex": BSD's form, also macOS's shasum --tag.
	bsdLine = regexp.MustCompile(`^(?i)(MD5|SHA-?1|SHA-?256|SHA-?384|SHA-?512)\s*\((.*)\)\s*=\s*([0-9a-f]+)$`)
	// "hex  name" or "hex *name": GNU's form; the name is optional.
	gnuLine = regexp.MustCompile(`^(?i)([0-9a-f]{32,128})(?:\s+\*?(.*))?$`)
	// "sha256:hex", "SHA-256: hex", "SHA256 hex".
	labelled = regexp.MustCompile(`^(?i)(MD5|SHA-?1|SHA-?256|SHA-?384|SHA-?512)(?:sum)?\s*[:=]?\s*([0-9a-f]+)$`)
)

// Parse reads one pasted checksum: a bare hash, a labelled hash, or one checksum-file line.
func Parse(text string) (Sum, bool) {
	text = strings.TrimSpace(text)
	if m := labelled.FindStringSubmatch(text); m != nil {
		algo, _ := byName(m[1])
		return fromHex(m[2], algo)
	}
	if m := bsdLine.FindStringSubmatch(text); m != nil {
		algo, _ := byName(m[1])
		return fromHex(m[3], algo)
	}
	if m := gnuLine.FindStringSubmatch(text); m != nil {
		return fromHex(m[1], "")
	}
	return Sum{}, false
}

// Find picks name's checksum out of a GNU- or BSD-style checksum file. A lone unnamed checksum
// is taken to be name's; if name is listed more than once, the strongest wins.
func Find(sums []byte, name string) (Sum, bool) {
	want := strings.ToLower(path.Base(name))
	var best, unnamed Sum
	count := 0
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-----") {
			continue // empty, comments, and PGP armor
		}
		var sum Sum
		var file string
		if m := bsdLine.FindStringSubmatch(line); m != nil {
			algo, _ := byName(m[1])
			s, ok := fromHex(m[3], algo)
			if !ok {
				continue
			}
			sum, file = s, m[2]
		} else if m := gnuLine.FindStringSubmatch(line); m != nil {
			s, ok := fromHex(m[1], "")
			if !ok {
				continue
			}
			sum, file = s, strings.TrimSpace(m[2])
		} else {
			continue
		}
		count++
		if file == "" {
			unnamed = sum
			continue
		}
		file = strings.TrimPrefix(strings.TrimPrefix(file, "*"), "./")
		if strings.ToLower(path.Base(file)) == want && sum.Better(best) {
			best = sum
		}
	}
	if !best.IsZero() {
		return best, true
	}
	if count == 1 && !unnamed.IsZero() {
		return unnamed, true
	}
	return Sum{}, false
}

// FromHeaders reads a whole-file checksum from Repr-Digest, Digest, x-goog-hash or Content-MD5.
// Content-Digest and compressed responses are ignored: those hash the bytes sent, not the file.
func FromHeaders(h http.Header) (Sum, bool) {
	if enc := h.Get("Content-Encoding"); enc != "" && enc != "identity" {
		return Sum{}, false
	}
	var best Sum
	consider := func(s Sum, ok bool) {
		if ok && s.Better(best) {
			best = s
		}
	}
	for _, header := range []string{"Repr-Digest", "Digest", "X-Goog-Hash"} {
		for _, value := range h.Values(header) {
			for item := range strings.SplitSeq(value, ",") {
				name, val, ok := strings.Cut(strings.TrimSpace(item), "=")
				if !ok {
					continue
				}
				algo, known := byName(name)
				if !known {
					continue
				}
				consider(fromBase64(val, algo))
			}
		}
	}
	if v := h.Get("Content-MD5"); v != "" {
		consider(fromBase64(v, MD5))
	}
	return best, !best.IsZero()
}

// ErrUnknownAlgo means the algorithm isn't supported.
var ErrUnknownAlgo = errors.New("unknown kind of checksum")

// File computes path's checksum in lower-case hex, calling progress (if non-nil) periodically
// with the bytes read so far.
func File(ctx context.Context, path string, algo Algo, progress func(read int64)) (string, error) {
	h := algo.new()
	if h == nil {
		return "", ErrUnknownAlgo
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 1<<20)
	var read int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			read += int64(n)
			if progress != nil {
				progress(read)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
