package protocol

import "os"

const specsDir = "schema/json"

func readSpecFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
