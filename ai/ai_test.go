package ai_test

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"

	"github.com/jordanbrauer/hex/ai"
)

// stubAgent is a minimal ai.Agent that records the calls it receives.
type stubAgent struct {
	generate func(context.Context, ai.Call) (*ai.Result, error)
	stream   func(context.Context, ai.StreamCall) (*ai.Result, error)
}

var _ ai.Agent = (*stubAgent)(nil)

func (s *stubAgent) Generate(ctx context.Context, c ai.Call) (*ai.Result, error) {
	if s.generate != nil {
		return s.generate(ctx, c)
	}

	return &ai.Result{}, nil
}

func (s *stubAgent) Stream(ctx context.Context, c ai.StreamCall) (*ai.Result, error) {
	if s.stream != nil {
		return s.stream(ctx, c)
	}

	return &ai.Result{}, nil
}

func TestDefault_nilBeforeSetDefault(t *testing.T) {
	ai.SetDefault(nil) // clean slate

	if got := ai.Default(); got != nil {
		t.Errorf("Default() = %v, want nil", got)
	}
}

func TestSetDefault_installsAgent(t *testing.T) {
	t.Cleanup(func() { ai.SetDefault(nil) })

	stub := &stubAgent{}
	ai.SetDefault(stub)

	if got := ai.Default(); got != stub {
		t.Errorf("Default() = %v, want stub", got)
	}
}

func TestGenerate_returnsErrWhenNoDefault(t *testing.T) {
	ai.SetDefault(nil)

	_, err := ai.Generate(context.Background(), ai.Call{Prompt: "hi"})
	if !errors.Is(err, ai.ErrNoDefaultAgent) {
		t.Errorf("Generate error = %v, want ErrNoDefaultAgent", err)
	}
}

func TestPrompt_routesToDefault(t *testing.T) {
	t.Cleanup(func() { ai.SetDefault(nil) })

	var seen ai.Call

	ai.SetDefault(&stubAgent{
		generate: func(_ context.Context, c ai.Call) (*ai.Result, error) {
			seen = c

			return &ai.Result{}, nil
		},
	})

	if _, err := ai.Prompt(context.Background(), "hello world"); err != nil {
		t.Fatalf("Prompt error = %v", err)
	}

	if seen.Prompt != "hello world" {
		t.Errorf("prompt = %q, want %q", seen.Prompt, "hello world")
	}
}

func TestStream_returnsErrWhenNoDefault(t *testing.T) {
	ai.SetDefault(nil)

	_, err := ai.Stream(context.Background(), ai.StreamCall{})
	if !errors.Is(err, ai.ErrNoDefaultAgent) {
		t.Errorf("Stream error = %v, want ErrNoDefaultAgent", err)
	}
}

// Compile-time assertions: type aliases remain aliases so hex/ai and
// fantasy values interchange without conversions.
func TestTypeAliasesAreAliases(t *testing.T) {
	var (
		_ ai.Agent    = (fantasy.Agent)(nil)
		_ ai.Provider = (fantasy.Provider)(nil)
		_ ai.Tool     = (fantasy.AgentTool)(nil)
		_             = ai.Call(fantasy.AgentCall{})
		_             = ai.Result(fantasy.AgentResult{})
	)
}
