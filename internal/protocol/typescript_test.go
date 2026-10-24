package protocol

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/probe"
)

var update = flag.Bool("update", false, "rewrite the extension's copy of the wire types")

// After changing a wire type: go test ./internal/protocol -run TypeScript -update
func TestTypeScriptTypesAreCurrent(t *testing.T) {
	path := filepath.Join("..", "..", "extension", "src", "shared", "wire.gen.ts")
	got, err := typescript([]named{
		{"Download", download{}},
		{"Check", check{}},
		{"Settings", settings{}},
		{"ProbeResult", probe.Result{}},
		{"Choice", probe.Choice{}},
		{"Subtitle", probe.Subtitle{}},
		{"Playlist", probe.Playlist{}},
		{"PlaylistFound", playlistFound{}},
		{"LinksFound", linksFound{}},
		{"Cookie", cookies.Cookie{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil || string(want) != got {
		t.Errorf("%s is out of date (%v); run: go test ./internal/protocol -run TypeScript -update", path, err)
	}
}

type named struct {
	name  string
	value any
}

// typescript writes an interface per type, field for field as encoding/json writes them.
func typescript(types []named) (string, error) {
	names := map[reflect.Type]string{}
	for _, n := range types {
		names[reflect.TypeOf(n.value)] = n.name
	}
	var b strings.Builder
	b.WriteString("// What Snag sends the extension. Made from the Go types by internal/protocol's\n")
	b.WriteString("// TestTypeScriptTypesAreCurrent; don't edit by hand.\n")
	for _, n := range types {
		fmt.Fprintf(&b, "\nexport interface %s {\n", n.name)
		typ := reflect.TypeOf(n.value)
		for i := range typ.NumField() {
			field := typ.Field(i)
			name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
			if !field.IsExported() || name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			tsType, err := tsTypeOf(field.Type, names)
			if err != nil {
				return "", fmt.Errorf("%s.%s: %w", n.name, field.Name, err)
			}
			optional := ""
			if strings.Contains(options, "omitempty") || strings.Contains(options, "omitzero") {
				optional = "?"
			}
			fmt.Fprintf(&b, "  %s%s: %s;\n", name, optional, tsType)
		}
		b.WriteString("}\n")
	}
	return b.String(), nil
}

func tsTypeOf(t reflect.Type, names map[reflect.Type]string) (string, error) {
	if name, ok := names[t]; ok {
		return name, nil
	}
	switch t.Kind() {
	case reflect.String:
		return "string", nil
	case reflect.Bool:
		return "boolean", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number", nil
	case reflect.Pointer:
		elem, err := tsTypeOf(t.Elem(), names)
		return elem + " | null", err
	case reflect.Slice:
		elem, err := tsTypeOf(t.Elem(), names)
		return elem + "[]", err
	case reflect.Map:
		elem, err := tsTypeOf(t.Elem(), names)
		return "Record<string, " + elem + ">", err
	}
	return "", fmt.Errorf("no TypeScript type for %s", t)
}
