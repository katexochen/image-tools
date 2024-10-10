package testing

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func FileDigest(path string) (string, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sha256 := sha256.New()
	if _, err := io.Copy(sha256, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum(nil)), nil
}

func MapDir(dir string) (map[string]string, error) {
	out := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		digest, err := FileDigest(path)
		if err != nil {
			return err
		}
		out[filepath.Base(path)] = digest
		return nil
	})
	return out, err
}

func MapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || va != vb {
			return false
		}
	}
	return true
}
