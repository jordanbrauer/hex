// Schema for hex/ai configuration.

// provider identifies which fantasy.Provider factory to construct.
// Consumer factories register a map of name → Factory in their
// hex/ai/provider.Provider.Factories; the value here selects one.
provider!: string & !=""

// model is the language-model identifier passed to the fantasy
// provider's LanguageModel method (e.g. "gpt-4o-mini", "claude-3-5-sonnet").
model!: string & !=""

// system_prompt is prepended to every call the default agent makes.
system_prompt?: string

settings?: {
	temperature?:       float & >=0.0 & <=2.0
	max_output_tokens?: int & >=1
	top_p?:             float & >=0.0 & <=1.0
	top_k?:             int & >=0
}

// Per-provider blocks. Only the block matching `provider` is read.
openai?: {
	api_key?:      string
	base_url?:     string
	organization?: string
	project?:      string
}

anthropic?: {
	api_key?:  string
	base_url?: string
}
