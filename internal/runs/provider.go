package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/heidaraliy/rune/internal/domain"
)

type Request struct {
	Run             domain.Run
	Task            domain.Entity
	ContextSnapshot string
}

type ArtifactInput struct {
	Kind        string
	Name        string
	MediaType   string
	Content     []byte
	Retention   string
	SecretState string
}

type Result struct {
	Summary   string
	Artifacts []ArtifactInput
}

type Provider interface {
	Execute(context.Context, Request) (Result, error)
}

type FakeProvider struct {
	Fail bool
}

func (p FakeProvider) Execute(ctx context.Context, request Request) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}
	if p.Fail {
		return Result{}, errors.New("fake provider requested failure")
	}
	var contextValue map[string]any
	if err := json.Unmarshal([]byte(request.ContextSnapshot), &contextValue); err != nil {
		return Result{}, fmt.Errorf("fake provider received invalid context: %w", err)
	}
	if strings.TrimSpace(request.Task.Title) == "" {
		return Result{}, errors.New("fake provider requires a task title")
	}
	content := fmt.Sprintf("# Local agent result\n\nTask: %s\n\nProvider: %s\n\nContext keys: %d\n", request.Task.Title, request.Run.Provider, len(contextValue))
	return Result{
		Summary: fmt.Sprintf("Fake provider completed %q", request.Task.Title),
		Artifacts: []ArtifactInput{{
			Kind:        "result",
			Name:        "result.md",
			MediaType:   "text/markdown",
			Content:     []byte(content),
			Retention:   "normal",
			SecretState: "clear",
		}},
	}, nil
}

func ProviderFor(name string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "fake", "local":
		return FakeProvider{}, nil
	case "fake-fail":
		return FakeProvider{Fail: true}, nil
	default:
		return nil, fmt.Errorf("unknown v2 run provider %q; use fake", name)
	}
}
