package yamlparser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileWithReaderValidatesPendingGraph(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "app.hyperbricks.yaml")
	partial := filepath.Join(root, "parts", "base.hyperbricks.yaml")
	sources := map[string][]byte{main: []byte("imports: [parts/base.hyperbricks.yaml]\npage:\n  - inherit: base\n  - route: hello\n"), partial: []byte("base:\n  - type: hypermedia\n  - title: Base\n")}
	read := func(path string) ([]byte, error) {
		b, ok := sources[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return b, nil
	}
	doc, err := LoadFileWithReader(main, Options{}, read)
	if err != nil {
		t.Fatal(err)
	}
	values, _, err := doc.MaterializeWithOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if values["page"].(map[string]interface{})["@type"] != "<HYPERMEDIA>" {
		t.Fatal(values)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("reader wrote files")
	}
	sources[partial] = []byte("imports: [../app.hyperbricks.yaml]\n")
	if _, err = LoadFileWithReader(main, Options{}, read); err == nil {
		t.Fatal("cycle accepted")
	}
	if _, err = LoadFileWithReader(main, Options{}, nil); err == nil {
		t.Fatal("nil reader accepted")
	}
}
