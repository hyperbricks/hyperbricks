package spaces

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalCreatorPreviewMatchesCMSAndDetectsChanges(t *testing.T) {
	s := testService(t)
	creator, err := NewCreator(s.module, s.dirs, nil)
	if err != nil {
		t.Fatal(err)
	}
	sources, err := creator.Sources()
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources %v %v", sources, err)
	}
	p, err := creator.Prepare("portfolio_page", "local", "Local", "local")
	if err != nil {
		t.Fatal(err)
	}
	files := p.Files()
	if len(files) != 3 {
		t.Fatal(files)
	}
	if _, err = os.Stat(files[0].Path); !os.IsNotExist(err) {
		t.Fatal("preview persisted instance")
	}
	files[0].After = "tampered"
	if p.Files()[0].After == "tampered" {
		t.Fatal("preview mutated write plan")
	}
	if err = p.Apply(); err != nil {
		t.Fatal(err)
	}
	c := catalogTest(t, s)
	snapshot, err := s.snapshot(c)
	if err != nil || len(snapshot.Spaces) != 1 || snapshot.Spaces[0].Name != "local" {
		t.Fatalf("CMS cannot see local creation: %+v %v", snapshot, err)
	}
	p, err = creator.Prepare("portfolio_page", "stale", "Stale", "stale")
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml")
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, f, string(b)+"\n# external change\n")
	if err = p.Apply(); err == nil {
		t.Fatal("stale creation accepted")
	}
	if _, err = os.Stat(p.Files()[0].Path); !os.IsNotExist(err) {
		t.Fatal("stale creation wrote instance")
	}
}
