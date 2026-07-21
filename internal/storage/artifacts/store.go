package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const DefaultMaxBytes int64 = 16 << 20

type Blob struct {
	SHA256     string
	StorageKey string
	SizeBytes  int64
	Created    bool
}

type Store struct {
	root     string
	maxBytes int64
}

func Open(root string) (*Store, error) {
	return OpenWithLimit(root, DefaultMaxBytes)
}

func OpenWithLimit(root string, maxBytes int64) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("artifact root is required")
	}
	if maxBytes <= 0 {
		return nil, errors.New("artifact max size must be positive")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact root: %w", err)
	}
	return &Store{root: root, maxBytes: maxBytes}, nil
}

func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *Store) Put(ctx context.Context, content []byte) (Blob, error) {
	if s == nil {
		return Blob{}, errors.New("artifact store is nil")
	}
	if err := ctx.Err(); err != nil {
		return Blob{}, err
	}
	if int64(len(content)) > s.maxBytes {
		return Blob{}, fmt.Errorf("artifact is %d bytes, limit is %d", len(content), s.maxBytes)
	}
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	key := filepath.ToSlash(filepath.Join("sha256", hash[:2], hash))
	path, err := s.safePath(key)
	if err != nil {
		return Blob{}, err
	}
	if info, err := os.Stat(path); err == nil {
		if info.Size() != int64(len(content)) {
			return Blob{}, fmt.Errorf("artifact hash collision at %s", key)
		}
		return Blob{SHA256: hash, StorageKey: key, SizeBytes: info.Size()}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Blob{}, fmt.Errorf("inspect artifact blob: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Blob{}, fmt.Errorf("create artifact shard: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rune-artifact-*")
	if err != nil {
		return Blob{}, fmt.Errorf("create artifact temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := writeContext(tmp, ctx, content); err != nil {
		_ = tmp.Close()
		return Blob{}, err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return Blob{}, fmt.Errorf("set artifact permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return Blob{}, fmt.Errorf("close artifact temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		if info, statErr := os.Stat(path); statErr == nil && info.Size() == int64(len(content)) {
			return Blob{SHA256: hash, StorageKey: key, SizeBytes: info.Size()}, nil
		}
		return Blob{}, fmt.Errorf("commit artifact blob: %w", err)
	}
	return Blob{SHA256: hash, StorageKey: key, SizeBytes: int64(len(content)), Created: true}, nil
}

func (s *Store) Read(ctx context.Context, storageKey string) ([]byte, error) {
	if s == nil {
		return nil, errors.New("artifact store is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.safePath(storageKey)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open artifact blob: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, s.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read artifact blob: %w", err)
	}
	if int64(len(data)) > s.maxBytes {
		return nil, fmt.Errorf("artifact blob exceeds configured limit of %d bytes", s.maxBytes)
	}
	return data, nil
}

func (s *Store) RemoveIfCreated(blob Blob) error {
	if s == nil || !blob.Created {
		return nil
	}
	path, err := s.safePath(blob.StorageKey)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove artifact blob: %w", err)
	}
	return nil
}

func (s *Store) safePath(storageKey string) (string, error) {
	key := filepath.Clean(filepath.FromSlash(strings.TrimSpace(storageKey)))
	if key == "." || key == ".." || filepath.IsAbs(key) || strings.HasPrefix(key, ".."+string(os.PathSeparator)) {
		return "", errors.New("invalid artifact storage key")
	}
	return filepath.Join(s.root, key), nil
}

func writeContext(file *os.File, ctx context.Context, content []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("write artifact temp file: %w", err)
	}
	return nil
}
