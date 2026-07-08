// Package featureflag is a thin wrapper around
// github.com/thomaspoignant/go-feature-flag (GOFF) that gives hex
// applications feature-flagging with rule-based targeting, percentage
// rollouts, and typed variations.
//
// See ADR-0013 for the wrap decision. Design summary:
//
//   - Type aliases (Client, Context, EvaluationContext) so consumers can
//     call every GOFF method through the alias.
//   - hex-owned constructors for the two common shapes: a file on disk
//     and a file in an embed.FS. Advanced retrievers (HTTP, S3, K8s)
//     are supplied by importing GOFF's retriever subpackage directly
//     and passing it via Options.Retrievers.
//   - Package-level convenience (SetDefault + Bool/Int/String/Float64/JSON)
//     mirroring hex/config and hex/i18n.
//
// Example (embed.FS):
//
//	//go:embed flags.yaml
//	var flagsFS embed.FS
//
//	client, err := featureflag.NewFromFS(flagsFS, "flags.yaml", featureflag.Options{
//	    PollingInterval: 30 * time.Second,
//	})
//	if err != nil { return err }
//	defer client.Close()
//
//	featureflag.SetDefault(client)
//
//	ctx := featureflag.NewContext("user-42").AddCustom("beta", "true")
//	if featureflag.Bool("new-checkout", ctx, false) {
//	    // ...
//	}
package featureflag

import (
	"context"
	"errors"
	"io/fs"
	"time"

	ffclient "github.com/thomaspoignant/go-feature-flag"
	"github.com/thomaspoignant/go-feature-flag/notifier"
	"github.com/thomaspoignant/go-feature-flag/retriever"
	"github.com/thomaspoignant/go-feature-flag/retriever/fileretriever"
	"github.com/thomaspoignant/go-feature-flag/utils/fflog"

	ffcontext "github.com/thomaspoignant/go-feature-flag/modules/core/ffcontext"
)

// Client is the type alias for GOFF's evaluation client. Consumers get the
// full upstream API through this alias (BoolVariation, IntVariation,
// AllFlagsState, RawVariation, etc.).
type Client = ffclient.GoFeatureFlag

// Context is the type alias for GOFF's evaluation context interface.
type Context = ffcontext.Context

// EvaluationContext is the concrete builder type consumers use to attach
// user attributes for targeting rules.
type EvaluationContext = ffcontext.EvaluationContext

// Retriever is the type alias for GOFF's Retriever interface. Consumers
// who want an unusual retriever (HTTP, S3, K8s, Postgres, etc.) can
// import GOFF's retriever subpackage and pass an instance directly.
type Retriever = retriever.Retriever

// Notifier is the type alias for GOFF's Notifier interface for
// flag-change hooks (webhook, log, custom).
type Notifier = notifier.Notifier

// Options tune a Client. Only the knobs hex applications typically touch
// are surfaced; consumers who need advanced upstream options can construct
// ffclient.Config directly and call ffclient.New.
type Options struct {
	// PollingInterval is how often retrievers re-fetch the flag file.
	// Zero uses GOFF's default (60s). Values below the GOFF minimum are
	// clamped upstream.
	PollingInterval time.Duration

	// StartWithRetrieverError, when true, lets the client start even if
	// the initial retrieve fails. Useful when the flag source is
	// intermittent and you want to fall back on defaults rather than
	// bail out at startup.
	StartWithRetrieverError bool

	// Retrievers lets consumers supply additional retrievers alongside
	// the primary one built by New*/NewFromFile/NewFromFS. Later
	// retrievers take precedence over earlier ones in GOFF's merge logic.
	Retrievers []Retriever

	// Notifiers registers flag-change listeners.
	Notifiers []Notifier

	// FileFormat overrides retriever content-type detection. Supported:
	// "yaml", "json", "toml". Empty means auto-detect from extension.
	FileFormat string
}

// New builds a Client from a full ffclient.Config. Use this when the
// upstream Config surface has knobs the hex helpers do not expose.
func New(cfg ffclient.Config) (*Client, error) {
	return ffclient.New(cfg)
}

// NewFromFile builds a Client backed by a flag file on disk.
func NewFromFile(path string, opts Options) (*Client, error) {
	if path == "" {
		return nil, errors.New("featureflag: path is required")
	}

	r := &fileretriever.Retriever{Path: path}

	return newClient(r, opts)
}

// NewFromFS builds a Client backed by a flag file in an fs.FS (typically
// an //go:embed FS). Flags are read once at start and are immutable at
// runtime — polling is disabled because embed.FS content cannot change.
func NewFromFS(fsys fs.FS, path string, opts Options) (*Client, error) {
	if fsys == nil {
		return nil, errors.New("featureflag: fs is required")
	}

	if path == "" {
		return nil, errors.New("featureflag: path is required")
	}

	// Read once so New(...) can validate the payload immediately.
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}

	// Polling is meaningless for a build-time immutable FS; force it
	// long so the retriever effectively runs only at startup.
	if opts.PollingInterval == 0 {
		opts.PollingInterval = 24 * time.Hour
	}

	return newClient(&fsRetriever{data: data, path: path}, opts)
}

// newClient assembles the ffclient.Config from a primary retriever plus
// hex-side Options and constructs a Client.
func newClient(primary Retriever, opts Options) (*Client, error) {
	cfg := ffclient.Config{
		Retriever:               primary,
		Retrievers:              opts.Retrievers,
		Notifiers:               opts.Notifiers,
		PollingInterval:         opts.PollingInterval,
		StartWithRetrieverError: opts.StartWithRetrieverError,
		FileFormat:              opts.FileFormat,
	}

	return ffclient.New(cfg)
}

// -- FS retriever ---------------------------------------------------------

// fsRetriever satisfies GOFF's Retriever interface using an in-memory byte
// slice. It ignores context cancellation and any input path — it just
// returns the pre-read bytes. Suitable for build-time immutable flag sets.
type fsRetriever struct {
	data []byte
	path string
}

func (r *fsRetriever) Retrieve(_ context.Context) ([]byte, error) {
	if len(r.data) == 0 {
		return nil, errors.New("featureflag: empty embedded flag file")
	}

	return r.data, nil
}

// SetLogger satisfies GOFF's optional retriever logger interface. We
// discard the logger — hex/log is the app-wide logger, not this retriever.
func (r *fsRetriever) SetLogger(_ *fflog.FFLogger) {}

// -- Context helpers ------------------------------------------------------

// NewContext returns an EvaluationContext keyed by userKey. Wrap in
// ContextWith to attach attributes with a fluent chain.
func NewContext(userKey string) EvaluationContext {
	return ffcontext.NewEvaluationContext(userKey)
}

// NewAnonymousContext returns an anonymous EvaluationContext for
// unauthenticated / pre-identification traffic.
func NewAnonymousContext(key string) EvaluationContext {
	return ffcontext.NewAnonymousEvaluationContext(key)
}

// ContextWith wraps an EvaluationContext with a fluent builder that
// makes attribute chains readable at call sites:
//
//	ctx := featureflag.ContextWith(featureflag.NewContext("u1")).
//		Set("beta", "true").
//		Set("region", "us").
//		Context()
//
// The underlying upstream API uses void mutators (AddCustomAttribute);
// this helper preserves the standard EvaluationContext type for hand-off
// while giving callers a fluent construction path.
func ContextWith(ec EvaluationContext) *ContextBuilder {
	return &ContextBuilder{ec: ec}
}

// ContextBuilder is a fluent-chain wrapper around EvaluationContext.
type ContextBuilder struct {
	ec EvaluationContext
}

// Set attaches a custom attribute for targeting rules and returns the
// builder for chaining.
func (b *ContextBuilder) Set(name string, value any) *ContextBuilder {
	b.ec.AddCustomAttribute(name, value)

	return b
}

// Context returns the underlying EvaluationContext ready to hand to a
// Variation call.
func (b *ContextBuilder) Context() EvaluationContext { return b.ec }
