package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/assets"
)

func TestDoctorRejectsRemovedEsbuildBinary(t *testing.T) {
	for _, value := range []string{`""`, "/usr/local/bin/esbuild"} {
		t.Run(value, func(t *testing.T) {
			root := t.TempDir()
			writeDoctorFixture(t, root, "demo", strings.TrimSpace(assets.VersionMD), "", fmt.Sprintf(`bundle:
  - type: esbuild
  - entry: resources/app.js
  - outfile: static/app.js
  - binary: %s
`, value))
			report, _, code := executeDoctorJSON(t, root, "--module", "demo")
			check := doctorCheckByID(t, report, "components.native_schema")
			if code == 0 || check.Status != doctorFail || !strings.Contains(check.Message, "esbuild binary is no longer supported") {
				t.Fatalf("removed compiler option was silently accepted: exit=%d check=%+v", code, check)
			}
		})
	}
}
