package spaces

import (
	"bytes"
	"fmt"
	"os"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// Creator provides local source authoring without exposing or enabling the HTTP
// editor. Callers must obtain filesystem write authorization from their user.
type Creator struct{ service *service }

func NewCreator(module string, directories map[string]string, config map[string]interface{}) (*Creator, error) {
	s, err := newService(module, shared.DefaultSpacesRoute, directories, editorOptions{Write: true}, false)
	if err != nil {
		return nil, err
	}
	s.config = config
	return &Creator{service: s}, nil
}
func (c *Creator) Sources() ([]Source, error) {
	catalog, err := c.service.catalog()
	if err != nil {
		return nil, err
	}
	snapshot, err := c.service.snapshot(catalog)
	return snapshot.Sources, err
}
func (c *Creator) Prepare(source, name, title, route string) (*CreatePlan, error) {
	catalog, err := c.service.catalog()
	if err != nil {
		return nil, err
	}
	p := &CreatePlan{service: c.service, catalog: catalog}
	if err := c.service.planCreate(catalog, Mutation{Source: source, Name: name, Title: title, Route: route}, p); err != nil {
		return nil, err
	}
	return p, nil
}

type CreateFile struct {
	Path   string `json:"path"`
	Before string `json:"before,omitempty"`
	After  string `json:"after"`
	old    []byte
}
type CreatePlan struct {
	service *service
	catalog *catalog
	changes []CreateFile
}

// Files returns a preview copy. Changing the preview cannot alter the operation.
func (p *CreatePlan) Files() []CreateFile { return append([]CreateFile(nil), p.changes...) }
func (p *CreatePlan) verify() error {
	if err := p.service.verify(p.catalog); err != nil {
		return err
	}
	for _, f := range p.changes {
		if _, err := contained(p.service.dirs["hyperbricks"], relative(p.service.dirs["hyperbricks"], f.Path)); err != nil {
			return err
		}
		b, err := os.ReadFile(f.Path)
		if f.old == nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("file already exists or cannot be created: %s", f.Path)
			}
		} else if err != nil || !bytes.Equal(b, f.old) {
			return conflict("file changed; prepare creation again")
		}
	}
	return nil
}
func (p *CreatePlan) Apply() error {
	if err := p.verify(); err != nil {
		return err
	}
	var written []string
	// Leaves precede their imports; retain completed files for explicit recovery.
	for _, f := range p.changes {
		if err := p.service.replaceFile(f.Path, []byte(f.After), f.old); err != nil {
			return fmt.Errorf("create %s: %w; completed files retained: %v", f.Path, err, written)
		}
		written = append(written, f.Path)
	}
	return nil
}
