package sdk

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ResolvePlayableURL uses yt-dlp to resolve a remote track/page URL into a direct media URL.
func ResolvePlayableURL(ctx context.Context, input string) (string, error) {
	cmd := exec.CommandContext(ctx, resolveTool("yt-dlp"), "-g", input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	url := strings.TrimSpace(stdout.String())
	if url == "" {
		return "", fmt.Errorf("yt-dlp: empty playable url")
	}
	lines := strings.Split(url, "\n")
	return strings.TrimSpace(lines[0]), nil
}
