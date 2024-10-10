package testing

import (
	"context"
	"testing"
)

func TestFetchCAS(t *testing.T) {
	if _, err := FetchCAS(context.Background(), "sha256:c2d8b1dd13a17d34ce7a5df069cefab38a2580d6857050eb6abab616211a40ad"); err != nil {
		t.Fatal(err)
	}
}
