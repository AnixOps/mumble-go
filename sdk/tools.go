package sdk

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
)

func resolveTool(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	// Check local tools directory (next to executable)
	exePath, err := os.Executable()
	if err == nil {
		localTool := filepath.Join(filepath.Dir(exePath), "tools", name)
		if runtime.GOOS == "windows" {
			localTool += ".exe"
		}
		if _, err := os.Stat(localTool); err == nil {
			return localTool
		}
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

// EnsureTool downloads tool if not found
func EnsureTool(name string) error {
	if _, err := exec.LookPath(name); err == nil {
		return nil
	}
	return downloadTool(name)
}

func downloadTool(name string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	toolsDir := filepath.Join(filepath.Dir(exePath), "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return fmt.Errorf("create tools dir: %w", err)
	}

	switch name {
	case "ffmpeg":
		return downloadFFmpeg(toolsDir)
	case "yt-dlp":
		return downloadYtDlp(toolsDir)
	}
	return fmt.Errorf("unknown tool: %s", name)
}

func downloadFFmpeg(dir string) error {
	// Download static ffmpeg from gyan.dev
	var url string
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "amd64" {
			url = "https://github.com/yt-dlp/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
		} else {
			url = "https://github.com/yt-dlp/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win32-gpl.zip"
		}
	case "linux":
		if runtime.GOARCH == "amd64" {
			url = "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz"
		} else {
			url = "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz"
		}
	default:
		return fmt.Errorf("unsupported platform for ffmpeg auto-download")
	}

	fmt.Printf("Downloading ffmpeg from %s...\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download ffmpeg: %w", err)
	}
	defer resp.Body.Close()

	tmpZip := filepath.Join(dir, "ffmpeg.zip")
	f, err := os.Create(tmpZip)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpZip)
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("save ffmpeg: %w", err)
	}

	// Extract
	return extractZip(tmpZip, dir, "ffmpeg.exe")
}

func downloadYtDlp(dir string) error {
	url := "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"
	if runtime.GOOS == "windows" {
		url += ".exe"
	}

	fmt.Printf("Downloading yt-dlp from %s...\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download yt-dlp: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(filepath.Join(dir, "yt-dlp"+exeExt()))
	if err != nil {
		return fmt.Errorf("create yt-dlp: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("save yt-dlp: %w", err)
	}
	os.Chmod(f.Name(), 0755)
	return nil
}

func exeExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func extractZip(zipPath, destDir, binaryName string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if path.Base(f.Name) == binaryName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			outFile := filepath.Join(destDir, binaryName)
			out, err := os.Create(outFile)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, rc)
			if err != nil {
				return err
			}
			os.Chmod(outFile, 0755)
			return nil
		}
	}
	return fmt.Errorf("binary %s not found in zip", binaryName)
}
