package component

import "golang.org/x/sys/windows"

func assetOwnershipBaseDirectory() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
}
