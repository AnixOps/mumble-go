package sdk

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func resolveTool(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			candidate := filepath.Join(localAppData, "Microsoft", "WinGet", "Links", name+".exe")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return name
}
