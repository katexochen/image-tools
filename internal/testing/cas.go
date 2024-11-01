package testing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
)

const (
	casURI = "ghcr.io/katexochen/image-tools/testdata-cas"
)

func FetchCAS(ctx context.Context, digest string) (string, error) {
	casLocalPath, err := findCASLocalPath()
	if err != nil {
		return "", fmt.Errorf("finding local CAS path: %w", err)
	}
	targetPath := filepath.Join(casLocalPath, strings.Replace(digest, ":", "-", 1))
	if _, err := os.Stat(casLocalPath); errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(casLocalPath, 0o755)
	}

	repo, err := remote.NewRepository(casURI)
	if err != nil {
		return "", fmt.Errorf("creating repository: %w", err)
	}
	repo.HandleWarning = func(warning remote.Warning) {
		fmt.Printf("Warning from %s: %s\n", repo.Reference.Repository, warning.Text)
	}

	desc, rc, err := oras.Fetch(ctx, repo.Blobs(), digest, oras.DefaultFetchOptions)
	if err != nil {
		return "", fmt.Errorf("fetching content: %w", err)
	}
	defer rc.Close()

	var target io.Writer
	var source io.Reader

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		// Verify the cached file for correctness.
		targetFile, err := os.OpenFile(targetPath, os.O_RDONLY, 0)
		if err != nil {
			return "", fmt.Errorf("opening file: %w", err)
		}
		defer targetFile.Close()
		source = targetFile
		target = io.Discard
	} else if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	} else {
		defer targetFile.Close()
		target = targetFile
		source = rc
	}

	vr := content.NewVerifyReader(source, desc)
	if _, err := io.Copy(target, vr); err != nil {
		return "", fmt.Errorf("writing content: %w", err)
	}
	if err := vr.Verify(); err != nil {
		return "", fmt.Errorf("verifying content: %w", err)
	}

	return targetPath, nil
}

func findCASLocalPath() (string, error) {
	subdir := "testdata-cas"
	path := "."
	for {
		if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
			break
		}
		pathAbs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("getting absolute path: %w", err)
		}
		if pathAbs == "/" {
			return "", errors.New("unable to find .git directory")
		}
		path = filepath.Join(path, "..")
	}
	pathAbs, err := filepath.Abs(filepath.Join(path, subdir))
	if err != nil {
		return "", fmt.Errorf("getting absolute path: %w", err)
	}
	return pathAbs, nil
}
