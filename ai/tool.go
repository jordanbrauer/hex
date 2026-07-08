package ai

import (
	"context"

	"charm.land/fantasy"
)

// Tool call and response types re-exported from fantasy.
type (
	ToolCall     = fantasy.ToolCall
	ToolResponse = fantasy.ToolResponse
	ToolInfo     = fantasy.ToolInfo
)

// NewTool constructs a typed agent tool. The input struct produces a
// JSON schema via reflection; the handler runs when the model calls
// the tool. Errors returned from the handler are surfaced to the
// model as a tool error response (allowing recovery / retry).
//
// Example:
//
//	type WeatherInput struct {
//	    Location string `json:"location" description:"City name"`
//	    Units    string `json:"units" enum:"celsius,fahrenheit"`
//	}
//
//	weather := ai.NewTool(
//	    "get_weather",
//	    "Look up the current weather for a location.",
//	    func(ctx context.Context, in WeatherInput, _ ai.ToolCall) (ai.ToolResponse, error) {
//	        temp := lookupTemp(in.Location, in.Units)
//	        return ai.ToolResponse{Type: "text", Content: temp}, nil
//	    },
//	)
func NewTool[Input any](
	name, description string,
	fn func(ctx context.Context, input Input, call ToolCall) (ToolResponse, error),
) Tool {
	return fantasy.NewAgentTool(name, description, fn)
}

// NewParallelTool is like NewTool but marks the tool as safe to
// execute concurrently with other parallel tools in the same
// generation step. Non-parallel tools always run sequentially.
func NewParallelTool[Input any](
	name, description string,
	fn func(ctx context.Context, input Input, call ToolCall) (ToolResponse, error),
) Tool {
	return fantasy.NewParallelAgentTool(name, description, fn)
}
