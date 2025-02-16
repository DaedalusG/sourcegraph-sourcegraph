// This package defines both streaming and non-streaming completion REST endpoints. Should probably be renamed "rest".
package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/completions/streaming/anthropic"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/completions/streaming/dotcom"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/completions/streaming/llmproxy"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/completions/streaming/openai"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/cody"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	streamhttp "github.com/sourcegraph/sourcegraph/internal/search/streaming/http"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// MaxRequestDuration is the maximum amount of time a request can take before
// being cancelled.
const MaxRequestDuration = time.Minute

// NewCompletionsStreamHandler is an http handler which streams back completions results.
func NewCompletionsStreamHandler(logger log.Logger, db database.DB) http.Handler {
	rl := NewRateLimiter(db, redispool.Store, RateLimitScopeCompletion)
	return &streamHandler{logger: logger, rl: rl}
}

type streamHandler struct {
	logger log.Logger
	rl     RateLimiter
}

func GetCompletionClient(endpoint, provider, accessToken, model string) (types.CompletionsClient, error) {
	switch provider {
	case "anthropic":
		return anthropic.NewAnthropicClient(httpcli.ExternalDoer, accessToken, model), nil
	case "openai":
		return openai.NewOpenAIClient(httpcli.ExternalDoer, accessToken, model), nil
	case dotcom.ProviderName:
		return dotcom.NewClient(httpcli.ExternalDoer, accessToken, model), nil
	case llmproxy.ProviderName:
		return llmproxy.NewClient(httpcli.ExternalDoer, endpoint, accessToken, model)
	default:
		return nil, errors.Newf("unknown completion stream provider: %s", provider)
	}
}

func GetCompletionsConfig() *schema.Completions {
	completionsConfig := conf.Get().Completions

	if completionsConfig.ChatModel == "" {
		completionsConfig.ChatModel = completionsConfig.Model
	}

	if completionsConfig.Provider == llmproxy.ProviderName && completionsConfig.Endpoint == "" {
		completionsConfig.Endpoint = llmproxy.DefaultEndpoint
	}

	// When the Completions is present always use it
	if completionsConfig != nil {
		return completionsConfig
	}

	// If App is running and there wasn't a completions config
	// use a provider that sends the request to dotcom
	if deploy.IsApp() {
		appConfig := conf.Get().App
		if appConfig == nil {
			return nil
		}
		// Only the Provider, Access Token and Enabled required to forward the request to dotcom
		return &schema.Completions{
			AccessToken: appConfig.DotcomAuthToken,
			Enabled:     len(appConfig.DotcomAuthToken) > 0,
			Provider:    dotcom.ProviderName,
		}
	}
	return nil
}

func (h *streamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), MaxRequestDuration)
	defer cancel()

	completionsConfig := GetCompletionsConfig()
	if completionsConfig == nil || !completionsConfig.Enabled {
		http.Error(w, "completions are not configured or disabled", http.StatusInternalServerError)
		return
	}

	if isEnabled := cody.IsCodyEnabled(ctx); !isEnabled {
		http.Error(w, "cody experimental feature flag is not enabled for current user", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, fmt.Sprintf("unsupported method %s", r.Method), http.StatusBadRequest)
		return
	}

	var requestParams types.ChatCompletionRequestParameters
	if err := json.NewDecoder(r.Body).Decode(&requestParams); err != nil {
		http.Error(w, "could not decode request body", http.StatusBadRequest)
		return
	}

	var err error
	ctx, done := Trace(ctx, "stream", completionsConfig.ChatModel).
		WithErrorP(&err).
		WithRequest(r).
		Build()
	defer done()

	completionClient, err := GetCompletionClient(
		completionsConfig.Endpoint,
		completionsConfig.Provider,
		completionsConfig.AccessToken,
		completionsConfig.ChatModel,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check rate limit.
	err = h.rl.TryAcquire(ctx)
	if err != nil {
		if unwrap, ok := err.(RateLimitExceededError); ok {
			respondRateLimited(w, unwrap)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	eventWriter, err := streamhttp.NewWriter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Always send a final done event so clients know the stream is shutting down.
	defer eventWriter.Event("done", map[string]any{})

	err = completionClient.Stream(ctx, requestParams, func(event types.ChatCompletionEvent) error { return eventWriter.Event("completion", event) })
	if err != nil {
		h.logger.Error("error while streaming completions", log.Error(err))
		eventWriter.Event("error", map[string]string{"error": err.Error()})
		return
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func respondRateLimited(w http.ResponseWriter, err RateLimitExceededError) {
	// Rate limit exceeded, write well known headers and return correct status code.
	w.Header().Set("x-ratelimit-limit", strconv.Itoa(err.Limit))
	w.Header().Set("x-ratelimit-remaining", strconv.Itoa(max(err.Limit-err.Used, 0)))
	w.Header().Set("retry-after", err.RetryAfter.Format(time.RFC1123))
	http.Error(w, err.Error(), http.StatusTooManyRequests)
}
