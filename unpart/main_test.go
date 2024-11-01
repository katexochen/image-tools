package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	it "github.com/katexochen/image-tools/internal/testing"
)

func TestUnpart(t *testing.T) {
	testCases := []struct {
		inFileDigest string
		wantOutFiles map[string]string
	}{
		{
			// Contrast podvm image, NixOS, built with Nix.
			inFileDigest: "sha256:f3a0566bcc49d70dc3318ab17ecba07b2f99215fd8c611c83db04bbe6a8ec4ae",
			wantOutFiles: map[string]string{
				"esp.part":         "sha256:24fcae6d5ca153c8a9d3f9dbfcb291c3b812b2beea172b505a381b630bbd2688",
				"root-verity.part": "sha256:f5f5606a437743039c548c82537210b830fc8f57dd613bc32dc0cf6e68a87edb",
				"root.part":        "sha256:e4784f2e7fccbc800159226646b7e7ba65f362d8863d37542bfa9d4f4dc6da73",
			},
		},
		// {
		// 	// Contrast AKS image, azurelinux. MBR, not supported yet.
		// 	inFileDigest: "sha256:757d0d6d766f8df076bbce920dcdce19dd43ed3a9136cfba5b7b8ddf0118823f",
		// 	wantOutFiles: map[string]string{},
		// },
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
				t.Fatal(fmt.Errorf("running unpart: %w", err))
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
