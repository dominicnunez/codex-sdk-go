package codex

import (
	"os"
	"path/filepath"
)

const specsDir = "../specs"

func readSpecFile(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join("..", path))
}
