package transcode

import (
	"bytes"
	"fmt"
	"os/exec"
)

// ConvertToWebP converts srcPath (e.g. a downloaded JPEG poster) to WebP at
// destPath. Used opportunistically when ffmpeg is available; callers should
// keep the source file as a fallback if this fails.
func ConvertToWebP(srcPath, destPath string) error {
	cmd := exec.Command("ffmpeg", "-y", "-i", srcPath, destPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg webp conversion failed: %w: %s", err, stderr.String())
	}
	return nil
}
