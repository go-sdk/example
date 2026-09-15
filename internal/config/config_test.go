package config

import "testing"

func TestGReturnsValueCopy(t *testing.T) {
	first := G()
	original := first.Storage.Root
	first.Storage.Root = "changed"
	if got := G().Storage.Root; got != original {
		t.Fatalf("G returned mutable global state: %q", got)
	}
}
