// Package ai is a thin, opinionated wrapper around charm.land/fantasy.
//
// The wrapper exists so hex applications get a single import path for
// LLM interactions, a container-bound default agent, and per-provider
// subpackages that stay decoupled from hex's core (a consumer using
// only OpenAI never compiles the anthropic-sdk-go tree, and vice
// versa).
//
// The public surface aliases fantasy's core types (Agent, Provider,
// Tool, Call, Result, Message, Usage, ToolChoice) so callers do not
// need to import fantasy directly. Package-level Generate and Stream
// operate on the "default agent" installed by the hex/ai/provider
// service provider during Bootstrap; consumers who want an explicit
// handle resolve *fantasy.Agent from the container under the key
// "ai".
package ai

import (
	"context"
	"errors"
	"sync/atomic"

	"charm.land/fantasy"
)

// Type aliases so callers do not need to import charm.land/fantasy
// side-by-side. Every alias is Go's = alias (identity, not a new
// type), so values flow freely between hex/ai and fantasy without
// conversions.
type (
	// Agent is the primary interaction point. Generate returns a full
	// AgentResult; Stream returns an incremental stream. Constructed
	// by fantasy.NewAgent(model, opts...).
	Agent = fantasy.Agent

	// Provider is the LLM backend (OpenAI, Anthropic, ...). Each
	// hex/ai/<name> subpackage constructs one from configuration.
	Provider = fantasy.Provider

	// LanguageModel is a single model instance from a Provider.
	LanguageModel = fantasy.LanguageModel

	// Tool is an agent tool. Build with NewTool[In, Out].
	Tool = fantasy.AgentTool

	// Call is a single request to an agent.
	Call = fantasy.AgentCall

	// StreamCall is a streaming request to an agent.
	StreamCall = fantasy.AgentStreamCall

	// Result is the full agent output including per-step traces.
	Result = fantasy.AgentResult

	// StepResult is one step in a multi-turn tool-using conversation.
	StepResult = fantasy.StepResult

	// Message is a single prompt message.
	Message = fantasy.Message

	// MessagePart is a piece of message content (text, tool call,
	// tool result, ...).
	MessagePart = fantasy.MessagePart

	// Usage counts input/output tokens for a run.
	Usage = fantasy.Usage

	// ToolChoice controls how the model selects tools.
	ToolChoice = fantasy.ToolChoice

	// AgentOption configures fantasy.NewAgent. Aliased so callers can
	// write ai.AgentOption without a second import.
	AgentOption = fantasy.AgentOption
)

// Common agent option helpers re-exported from fantasy for parity.
var (
	// WithSystemPrompt sets the system prompt for every call the agent
	// makes.
	WithSystemPrompt = fantasy.WithSystemPrompt

	// WithTools attaches tools to the agent.
	WithTools = fantasy.WithTools
)

// NewAgent constructs an agent for the given LanguageModel. Wraps
// fantasy.NewAgent verbatim; provided here so callers stay inside the
// hex/ai namespace.
func NewAgent(model LanguageModel, opts ...AgentOption) Agent {
	return fantasy.NewAgent(model, opts...)
}

// current holds the default agent installed by hex/ai/provider. Nil
// until Bootstrap runs (or the consumer calls SetDefault manually).
var current atomic.Pointer[agentBox]

// agentBox wraps Agent so atomic.Pointer works with an interface value.
type agentBox struct{ Agent }

// ErrNoDefaultAgent is returned by Generate/Stream/Default when no
// default agent has been installed. hex/ai/provider installs one
// during its Register phase; callers that use the package-level
// helpers before Bootstrap must SetDefault themselves.
var ErrNoDefaultAgent = errors.New("ai: no default agent (install hex/ai/provider or call ai.SetDefault)")

// SetDefault installs a as the process-wide default agent. Safe to
// call from any goroutine; last write wins.
func SetDefault(a Agent) {
	if a == nil {
		current.Store(nil)

		return
	}

	current.Store(&agentBox{Agent: a})
}

// Default returns the current default agent, or nil if none is
// installed.
func Default() Agent {
	box := current.Load()
	if box == nil {
		return nil
	}

	return box.Agent
}

// Generate runs a Call on the default agent. Returns ErrNoDefaultAgent
// if no default is installed.
func Generate(ctx context.Context, call Call) (*Result, error) {
	a := Default()
	if a == nil {
		return nil, ErrNoDefaultAgent
	}

	return a.Generate(ctx, call)
}

// Prompt is a shorthand for Generate with just a prompt string.
func Prompt(ctx context.Context, prompt string) (*Result, error) {
	return Generate(ctx, Call{Prompt: prompt})
}

// Stream runs a StreamCall on the default agent. Returns
// ErrNoDefaultAgent if no default is installed.
func Stream(ctx context.Context, call StreamCall) (*Result, error) {
	a := Default()
	if a == nil {
		return nil, ErrNoDefaultAgent
	}

	return a.Stream(ctx, call)
}
