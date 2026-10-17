package component

import "path/filepath"

// Ownership describes published files shared by every process for one output
// root. It must not move when a process changes its build-cache environment.
func assetOwnershipDirectory(kind string) (string, error) {
	base, err := assetOwnershipBaseDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "hyperbricks", kind), nil
}
