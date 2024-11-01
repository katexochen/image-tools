package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	it "github.com/katexochen/image-tools/internal/testing"
)

func TestInitrdsep(t *testing.T) {
	testCases := []struct {
		inFileDigest string
		wantOutFiles map[string]string
	}{
		{
			// initrd from a Constrellation image, GCP, built with mkosi.
			inFileDigest: "sha256:c2d8b1dd13a17d34ce7a5df069cefab38a2580d6857050eb6abab616211a40ad",
			wantOutFiles: map[string]string{
				"initrd_0": "sha256:83b773cdb11236220574f90f1e7626fcaa5c3b15cd1ba8f2c2faf354aab4d3cf",
				// Why does the initrd bundle contain the same archive twice?
				"initrd_1": "sha256:83b773cdb11236220574f90f1e7626fcaa5c3b15cd1ba8f2c2faf354aab4d3cf",
				"initrd_2": "sha256:1b938261099a585f6e67af7543a38c91ad758f169c45484b7a8cc8cd0197f8b9",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.inFileDigest, func(t *testing.T) {
			inPath, err := it.FetchCAS(context.Background(), tc.inFileDigest)
			if err != nil {
				t.Fatal(err)
			}
			os.Args[1] = inPath
			tmpDir := t.TempDir()

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatal(err)
			}

			if err := run(); err != nil {
				t.Fatal(fmt.Errorf("running initrdsep: %w", err))
			}

			outFiles, err := it.MapDir(tmpDir)
			if err != nil {
				t.Fatal(fmt.Errorf("mapping directory: %w", err))
			}

			if !it.MapsEqual(outFiles, tc.wantOutFiles) {
				t.Fatalf("got %v, want %v", outFiles, tc.wantOutFiles)
			}
		})
	}
}
