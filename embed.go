package navo

import "os"

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}
