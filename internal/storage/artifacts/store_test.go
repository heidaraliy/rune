package artifacts

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPutIsContentAddressedAndDeduplicated(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("hello artifact")
	first, err := store.Put(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Put(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 != second.SHA256 || !first.Created || second.Created {
		t.Fatalf("blobs = %#v / %#v", first, second)
	}
	if filepath.Base(first.StorageKey) != first.SHA256 {
		t.Fatalf("storage key = %q", first.StorageKey)
	}
	got, err := store.Read(context.Background(), first.StorageKey)
	if err != nil || string(got) != string(content) {
		t.Fatalf("read = %q, err=%v", got, err)
	}
}

func TestPutEnforcesSizeAndRejectsUnsafeKeys(t *testing.T) {
	store, err := OpenWithLimit(t.TempDir(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), []byte("12345")); err == nil {
		t.Fatal("oversized artifact should fail")
	}
	if _, err := store.Read(context.Background(), "../outside"); err == nil {
		t.Fatal("path traversal should fail")
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
}
