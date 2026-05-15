package core

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

const (
	idAlphabet      = "0123456789abcdefghijklmnopqrstuvwxyz"
	internalIDLen   = 8
	minDisplayIDLen = 3
)

func NewID(existing map[string]bool) (string, error) {
	limit := big.NewInt(int64(len(idAlphabet)))
	for attempts := 0; attempts < 10000; attempts++ {
		var out strings.Builder
		for out.Len() < internalIDLen {
			n, err := rand.Int(rand.Reader, limit)
			if err != nil {
				return "", err
			}
			out.WriteByte(idAlphabet[n.Int64()])
		}
		id := out.String()
		if !existing[id] {
			existing[id] = true
			return id, nil
		}
	}
	return "", errors.New("could not generate a unique rune id")
}

func ShortestPrefixes(items []*Item) map[string]string {
	counts := make(map[string]int)
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		for n := minDisplayIDLen; n <= len(item.ID); n++ {
			counts[item.ID[:n]]++
		}
	}
	result := make(map[string]string)
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		prefix := item.ID
		for n := minDisplayIDLen; n <= len(item.ID); n++ {
			candidate := item.ID[:n]
			if counts[candidate] == 1 {
				prefix = candidate
				break
			}
		}
		result[item.ID] = prefix
	}
	return result
}

type AmbiguousIDError struct {
	Prefix  string
	Matches []*Item
}

func (e AmbiguousIDError) Error() string {
	return fmt.Sprintf("ambiguous id: %s", e.Prefix)
}

func ResolvePrefix(items []*Item, prefix string) (*Item, error) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil, errors.New("empty id")
	}
	var matches []*Item
	for _, item := range items {
		if strings.HasPrefix(item.ID, prefix) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no item matches id %q", prefix)
	case 1:
		return matches[0], nil
	default:
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].DisplayID == matches[j].DisplayID {
				return matches[i].Title < matches[j].Title
			}
			return matches[i].DisplayID < matches[j].DisplayID
		})
		return nil, AmbiguousIDError{Prefix: prefix, Matches: matches}
	}
}

func ApplyDisplayIDs(items []*Item) {
	prefixes := ShortestPrefixes(items)
	for _, item := range items {
		item.DisplayID = prefixes[item.ID]
		if item.DisplayID == "" && item.ID != "" {
			if len(item.ID) < minDisplayIDLen {
				item.DisplayID = item.ID
			} else {
				item.DisplayID = item.ID[:minDisplayIDLen]
			}
		}
	}
}
