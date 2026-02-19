package importer

import (
	"bytes"
	"fmt"
	"os/exec"
)

// DecompressLZFSE decompresses an LZFSE-compressed file using the lzfse CLI tool.
// Returns the decompressed bytes.
func DecompressLZFSE(path string) ([]byte, error) {
	cmd := exec.Command("lzfse", "-decode", "-i", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("lzfse decode %s: %w (stderr: %s)", path, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
