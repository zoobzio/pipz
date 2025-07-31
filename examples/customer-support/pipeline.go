package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zoobzio/pipz"
)

// Route name constants.
const (
	routeSimple   = "simple"
	routeComplex  = "complex"
	routeNormal   = "normal"
	routeCritical = "critical"
	routeAngry    = "angry"
)

// ============================================================================.
// CUSTOMER SUPPORT CHATBOT PIPELINE - DEMONSTRATION EXAMPLE.
// ============================================================================.
// This example shows how to build a production-grade AI support system that.
// evolves from a simple MVP to a sophisticated multi-provider solution.
//
// WARNING: This uses mock AI services for demonstration. In production, you
// would integrate with real AI providers (OpenAI, Anthropic, etc.)
//
// The example demonstrates:.
// - Progressive enhancement through feature flags.
// - Cost optimization through intelligent routing.
// - Resilience through multi-provider fallback.
// - Performance optimization through racing.
// - Priority handling based on sentiment.

// ============================================================================.
// SPRINT 1: MVP - Just Answer Questions (Week 1).
// ============================================================================.
// CEO: "Everyone has AI support now. We need it TODAY! Just make it work!"
// Requirements:.
// - Take customer query.
// - Get AI response.
// - Show it to customer.
//
// Dev: "Done! Ship it! 🚀".

var (
	// ValidateQuery ensures we have a valid query to process.
	// Added: Sprint 1 - Basic validation.
	ValidateQuery = pipz.Apply(ProcessorValidateQuery, func(_ context.Context, q SupportQuery) (SupportQuery, error) {
		if strings.TrimSpace(q.Query) == "" {
			return q, errors.New("empty query")
		}
		if len(q.Query) > 1000 {
			return q, errors.New("query too long")
		}
		q.ProcessingLog = append(q.ProcessingLog, "validated")
		return q, nil
	})

	// CallGPT4 sends the query to GPT-4.
	// Added: Sprint 1 - Our only AI provider (for now).
	CallGPT4 = pipz.Apply(ProcessorCallGPT4, func(ctx context.Context, q SupportQuery) (SupportQuery, error) {
		start := time.Now()
		result, err := GPT4Service.Respond(ctx, q)
		if err != nil {
			return q, fmt.Errorf("gpt4 error: %w", err)
		}
		result.ProcessingLog = append(result.ProcessingLog, fmt.Sprintf("gpt4: %v", time.Since(start)))
		return result, nil
	})

	// FormatResponse adds a friendly wrapper to the AI response.
	// Added: Sprint 1 - Make it look nice.
	FormatResponse = pipz.Transform(ProcessorFormatResponse, func(_ context.Context, q SupportQuery) SupportQuery {
		if !strings.HasSuffix(q.Response, "?") {
			q.Response += "\n\nIs there anything else I can help you with today?"
		}
		q.ProcessingLog = append(q.ProcessingLog, "formatted")
		return q
	})
)

// ============================================================================.
// SPRINT 3: Cost Crisis (Week 3).
// ============================================================================.
// CFO storms into standup: "We spent $50,000 last month on AI costs! FIX THIS NOW!".
// Analysis: 70% of queries are simple "where is my order #12345?".
// Solution: Route simple queries to cheaper GPT-3.5
//
// Dev: "Why are we using GPT-4 to answer 'where is my order?'... 🤦"

var (
	// ClassifyQuery determines the type and complexity of the query.
	// Added: Sprint 3 - Need to route based on query type.
	ClassifyQuery = pipz.Transform(ProcessorClassifyQuery, func(_ context.Context, q SupportQuery) SupportQuery {
		query := strings.ToLower(q.Query)

		// Detect query type.
		switch {
		case strings.Contains(query, "order") || strings.Contains(query, "package") || strings.Contains(query, "delivery"):
			if strings.Contains(query, "where") || strings.Contains(query, "status") || strings.Contains(query, "track") || strings.Contains(query, "arrived") {
				q.QueryType = QueryTypeOrderStatus
			}
		case strings.Contains(query, "refund") || strings.Contains(query, "return") || strings.Contains(query, "money back"):
			q.QueryType = QueryTypeRefund
		case strings.Contains(query, "how") || strings.Contains(query, "work") || strings.Contains(query, "use"):
			q.QueryType = QueryTypeProduct
		case strings.Contains(query, "error") || strings.Contains(query, "api") || strings.Contains(query, "bug"):
			q.QueryType = QueryTypeTechnical
		case strings.Contains(query, "terrible") || strings.Contains(query, "awful") || strings.Contains(query, "worst"):
			q.QueryType = QueryTypeComplaint
		default:
			q.QueryType = QueryTypeUnknown
		}

		q.ProcessingLog = append(q.ProcessingLog, fmt.Sprintf("classified: %s", q.QueryType))
		return q
	})

	// ExtractOrderID pulls order IDs from queries for faster responses.
	// Added: Sprint 3 - Pre-extract data for better responses.
	ExtractOrderID = pipz.Transform(ProcessorExtractOrderID, func(_ context.Context, q SupportQuery) SupportQuery {
		// Look for patterns like ORD-12345 or #12345.
		words := strings.Fields(q.Query)
		for _, word := range words {
			if strings.HasPrefix(word, "ORD-") || strings.HasPrefix(word, "#") {
				q.OrderID = strings.TrimPrefix(strings.TrimPrefix(word, "#"), "ORD-")
				q.ProcessingLog = append(q.ProcessingLog, fmt.Sprintf("order_id: %s", q.OrderID))
				break
			}
		}
		return q
	})

	// CallGPT35 uses the cheaper GPT-3.5 model.
	// Added: Sprint 3 - 93% cheaper for simple queries!.
	CallGPT35 = pipz.Apply(ProcessorCallGPT35, func(ctx context.Context, q SupportQuery) (SupportQuery, error) {
		start := time.Now()
		result, err := GPT35Service.Respond(ctx, q)
		if err != nil {
			return q, fmt.Errorf("gpt3.5 error: %w", err)
		}
		result.ProcessingLog = append(result.ProcessingLog, fmt.Sprintf("gpt3.5: %v", time.Since(start)))
		return result, nil
	})

	// CheckCache looks for cached responses to common queries.
	// Added: Sprint 3 - Same questions asked 100s of times.
	CheckCache = pipz.Apply("check_cache", func(_ context.Context, q SupportQuery) (SupportQuery, error) {
		if cached, found := ResponseCache.Get(q.Query); found {
			q.Response = cached.Response
			q.Provider = cached.Provider + "-cached"
			q.ResponseTime = 1 * time.Millisecond // Near instant
			q.Cost = 0                            // Cached responses are free!
			q.CacheHit = true
			q.ProcessingLog = append(q.ProcessingLog, "cache: hit")
			return q, nil
		}
		q.ProcessingLog = append(q.ProcessingLog, "cache: miss")
		return q, nil
	})

	// UpdateCache stores responses for future use.
	// Added: Sprint 3 - Cache common responses.
	UpdateCache = pipz.Effect(ProcessorUpdateCache, func(_ context.Context, q SupportQuery) error {
		if !q.CacheHit && q.Response != "" {
			ResponseCache.Set(q.Query, q.Response, q.Provider)
		}
		return nil
	})

	// SimpleQueryPipeline handles basic queries with GPT-3.5.
	// Added: Sprint 3 - Optimized for cost.
	SimpleQueryPipeline = pipz.NewSequence[SupportQuery](
		PipelineSimpleQuery,
		CheckCache,
		pipz.Apply("process_or_cache", func(ctx context.Context, q SupportQuery) (SupportQuery, error) {
			if q.CacheHit {
				return q, nil // Already have response from cache
			}
			// Not cached, need to call GPT-3.5
			return GPT35Service.Respond(ctx, q)
		}),
		UpdateCache,
		FormatResponse,
	)

	// ComplexQueryPipeline uses GPT-4 for complex queries.
	// Added: Sprint 3 - Keep quality for complex issues.
	ComplexQueryPipeline = pipz.NewSequence[SupportQuery](
		PipelineComplexQuery,
		CallGPT4,
		FormatResponse,
	)
)

// ============================================================================.
// SPRINT 5: The Great Outage (Week 5).
// ============================================================================.
// 3 AM Saturday: OpenAI goes down. Support queue explodes to 500+ tickets.
// On-call dev: "We have no fallback! HELP!".
// Solution: Add Claude as backup, local model as last resort.
//
// Dev: "Maybe we should have thought about this earlier... ☕😴"

var (
	// CallClaude uses Claude as a fallback provider.
	// Added: Sprint 5 - Backup when OpenAI is down.
	CallClaude = pipz.Apply(ProcessorCallClaude, func(ctx context.Context, q SupportQuery) (SupportQuery, error) {
		start := time.Now()
		result, err := ClaudeService.Respond(ctx, q)
		if err != nil {
			return q, fmt.Errorf("claude error: %w", err)
		}
		result.ProcessingLog = append(result.ProcessingLog, fmt.Sprintf("claude: %v", time.Since(start)))
		q.Metrics.FallbackUsed = true
		return result, nil
	})

	// CallLocalModel uses our self-hosted model as last resort.
	// Added: Sprint 5 - When all else fails.
	CallLocalModel = pipz.Apply(ProcessorCallLocalModel, func(ctx context.Context, q SupportQuery) (SupportQuery, error) {
		start := time.Now()
		result, err := LocalModelService.Respond(ctx, q)
		if err != nil {
			return q, fmt.Errorf("local model error: %w", err)
		}
		result.ProcessingLog = append(result.ProcessingLog, fmt.Sprintf("local: %v", time.Since(start)))
		q.Metrics.FallbackUsed = true
		return result, nil
	})

	// GPTWithFallback tries GPT-4, then GPT-3.5.
	// Added: Sprint 5 - First level fallback.
	GPTWithFallback = pipz.NewFallback[SupportQuery](
		ConnectorGPTFallback,
		CallGPT4,
		CallGPT35,
	)

	// MultiProviderFallback provides complete fallback chain.
	// Added: Sprint 5 - Never let support go down!.
	MultiProviderFallback = pipz.NewFallback[SupportQuery](
		ConnectorMultiProvider,
		GPTWithFallback, // Try GPT models first
		pipz.NewFallback[SupportQuery](
			"claude_local_fallback",
			CallClaude,     // Then Claude
			CallLocalModel, // Finally our local model
		),
	)

	// ComplexQueryWithFallback handles complex queries with resilience.
	// Added: Sprint 5 - Complex queries need fallback too.
	ComplexQueryWithFallback = pipz.NewSequence[SupportQuery](
		"complex_with_fallback",
		MultiProviderFallback,
		FormatResponse,
	)
)

// ============================================================================.
// SPRINT 7: Speed Wars (Week 7).
// ============================================================================.
// CMO: "TechCrunch says our competitor has 500ms response times!.
//       Customers are tweeting that we're slow! This is a PR disaster!".
// Current: 2-3 seconds average.
// Solution: Race providers, take whoever responds first.
//
// Dev: "So we're literally racing AI providers now? Sure, why not... 🏁"

var (
	// AnalyzeSentiment determines customer emotion.
	// Added: Sprint 7 - Need to detect angry customers.
	AnalyzeSentiment = pipz.Transform(ProcessorAnalyzeSentiment, func(_ context.Context, q SupportQuery) SupportQuery {
		query := strings.ToLower(q.Query)

		// Simple sentiment analysis.
		angryWords := []string{"furious", "angry", "disgusted", "horrible", "worst", "terrible", "unacceptable"}
		negativeWords := []string{"disappointed", "unhappy", "frustrated", "annoyed", "problem", "issue"}
		positiveWords := []string{"great", "wonderful", "excellent", "happy", "thanks", "appreciate"}

		angryCount := 0
		negativeCount := 0
		positiveCount := 0

		for _, word := range angryWords {
			if strings.Contains(query, word) {
				angryCount++
			}
		}
		for _, word := range negativeWords {
			if strings.Contains(query, word) {
				negativeCount++
			}
		}
		for _, word := range positiveWords {
			if strings.Contains(query, word) {
				positiveCount++
			}
		}

		// ALL CAPS = angry.
		if strings.ToUpper(q.Query) == q.Query && len(q.Query) > 20 {
			angryCount += 2
		}

		// Multiple exclamation marks = urgent/angry.
		if strings.Count(q.Query, "!") > 2 {
			angryCount++
		}

		switch {
		case angryCount > 0:
			q.Sentiment = SentimentAngry
		case negativeCount > positiveCount:
			q.Sentiment = SentimentNegative
		case positiveCount > 0:
			q.Sentiment = SentimentPositive
		default:
			q.Sentiment = SentimentNeutral
		}

		q.ProcessingLog = append(q.ProcessingLog, fmt.Sprintf("sentiment: %s", q.Sentiment))
		return q
	})

	// DetermineUrgency sets the urgency level.
	// Added: Sprint 7 - Prioritize urgent queries.
	DetermineUrgency = pipz.Transform(ProcessorDetermineUrgency, func(_ context.Context, q SupportQuery) SupportQuery {
		query := strings.ToUpper(q.Query)

		// Check for urgent keywords.
		urgentPhrases := []string{"URGENT", "ASAP", "EMERGENCY", "IMMEDIATELY", "RIGHT NOW", "TODAY"}
		criticalPhrases := []string{"WEDDING", "FUNERAL", "BIRTHDAY", "ANNIVERSARY", "TOMORROW"}

		hasUrgent := false
		hasCritical := false

		for _, phrase := range urgentPhrases {
			if strings.Contains(query, phrase) {
				hasUrgent = true
				break
			}
		}

		for _, phrase := range criticalPhrases {
			if strings.Contains(query, phrase) {
				hasCritical = true
				break
			}
		}

		// Determine urgency.
		switch {
		case hasCritical || (hasUrgent && q.Sentiment == SentimentAngry):
			q.Urgency = UrgencyCritical
		case hasUrgent || q.Sentiment == SentimentAngry:
			q.Urgency = UrgencyHigh
		case q.QueryType == QueryTypeComplaint:
			q.Urgency = UrgencyHigh
		case q.QueryType == QueryTypeOrderStatus:
			q.Urgency = UrgencyNormal
		default:
			q.Urgency = UrgencyNormal
		}

		q.ProcessingLog = append(q.ProcessingLog, fmt.Sprintf("urgency: %s", q.Urgency))
		return q
	})

	// SpeedRace races all providers for fastest response.
	// Added: Sprint 7 - May the fastest AI win!.
	// Before: Sequential fallback = 4.5s worst case (try each provider)
	// After: Race pattern = 800ms average (first to respond wins!).
	// Performance gain: 82% faster for urgent queries!.
	SpeedRace = pipz.NewRace[SupportQuery](
		ConnectorSpeedRace,
		CallGPT35,      // Usually fastest (~800ms)
		CallClaude,     // Sometimes faster (~1.2s)
		CallLocalModel, // Consistent latency (~1.5s)
	// Note: Removed GPT-4 from race - too slow (2s).
	)

	// UrgentQueryPipeline handles urgent queries with speed.
	// Added: Sprint 7 - Speed matters for angry customers.
	UrgentQueryPipeline = pipz.NewSequence[SupportQuery](
		PipelineUrgentQuery,
		SpeedRace,
		FormatResponse,
	)
)

// ============================================================================.
// SPRINT 9: Priority Lanes (Week 9).
// ============================================================================.
// Data analyst: "Angry customers who wait become chargebacks.
//               Each chargeback costs us $35 plus the lost sale!".
// Solution: Detect sentiment, route angry+urgent to fastest path.
//
// Dev: "So we're building HOV lanes for angry customers? Makes sense... 🚦"

var (
	// ClassificationPipeline analyzes queries in detail.
	// Added: Sprint 9 - Full analysis pipeline.
	ClassificationPipeline = pipz.NewSequence[SupportQuery](
		PipelineClassification,
		ClassifyQuery,
		ExtractOrderID,
		AnalyzeSentiment,
		DetermineUrgency,
	)

	// RouteByUrgency directs queries based on urgency and sentiment.
	// Added: Sprint 9 - Smart routing for better CX.
	RouteByUrgency = pipz.NewSwitch[SupportQuery](
		ConnectorRouteByUrgency,
		func(_ context.Context, q SupportQuery) string {
			// Critical always gets fastest path.
			if q.Urgency == UrgencyCritical {
				return routeCritical
			}

			// Angry customers get priority.
			if q.Sentiment == SentimentAngry {
				return routeAngry
			}

			// Route by query type.
			switch q.QueryType {
			case QueryTypeOrderStatus:
				return routeSimple
			case QueryTypeRefund, QueryTypeComplaint:
				return routeComplex
			default:
				return routeNormal
			}
		},
	).
		AddRoute(routeCritical, UrgentQueryPipeline).     // Race for speed
		AddRoute(routeAngry, UrgentQueryPipeline).        // Race for speed
		AddRoute(routeSimple, SimpleQueryPipeline).       // Cached + GPT-3.5
		AddRoute(routeComplex, ComplexQueryWithFallback). // GPT-4 with fallback
		AddRoute(routeNormal, ComplexQueryWithFallback)   // Default path
)

// ============================================================================.
// SPRINT 11: Production Ready (Week 11).
// ============================================================================.
// CTO: "We're going viral! I need metrics, safety checks, and monitoring!".
// Also CTO: "And make sure it handles Black Friday traffic!".
// Solution: Add all the production bells and whistles.
//
// Dev: "From MVP to enterprise-grade in 11 weeks. We did it! 🎉"

var (
	// CheckSafety ensures responses are appropriate.
	// Added: Sprint 11 - Legal compliance.
	CheckSafety = pipz.Apply(ProcessorCheckSafety, func(_ context.Context, q SupportQuery) (SupportQuery, error) {
		// Check for sensitive data in response.
		response := strings.ToLower(q.Response)
		if strings.Contains(response, "password") ||
			strings.Contains(response, "credit card") ||
			strings.Contains(response, "ssn") {
			return q, errors.New("response contains sensitive data")
		}

		q.ProcessingLog = append(q.ProcessingLog, "safety: passed")
		return q, nil
	})

	// LogMetrics records performance metrics.
	// Added: Sprint 11 - Need visibility.
	LogMetrics = pipz.Effect(ProcessorLogMetrics, func(_ context.Context, q SupportQuery) error {
		q.Metrics.TotalTime = time.Since(q.Timestamp)
		MetricsCollector.Record(q)

		// Log for debugging.
		fmt.Printf("[Query %s] Provider: %s, Time: %v, Cost: $%.4f, Cache: %v\n",
			q.ID, q.Provider, q.ResponseTime, q.Cost, q.CacheHit)

		return nil
	})

	// ErrorRecovery handles pipeline failures gracefully.
	// Added: Sprint 11 - Never show errors to customers.
	ErrorRecovery = pipz.Apply("error_recovery", func(_ context.Context, err *pipz.Error[SupportQuery]) (*pipz.Error[SupportQuery], error) {
		// Provide a graceful fallback response.
		err.InputData.Response = "I apologize, but I'm having trouble processing your request right now. Please try again in a moment, or contact us directly at support@example.com for immediate assistance."
		err.InputData.ProcessingLog = append(err.InputData.ProcessingLog, fmt.Sprintf("error: %v", err.Err))

		// Log the error for monitoring.
		fmt.Printf("[ERROR] Query %s failed: %v at %v\n", err.InputData.ID, err.Err, err.Path)

		return err, nil
	})

	// TimeoutWrapper adds timeout protection.
	// Added: Sprint 11 - Don't let slow APIs block everything.
	TimeoutWrapper = func(processor pipz.Chainable[SupportQuery]) pipz.Chainable[SupportQuery] {
		return pipz.NewTimeout(
			"timeout_"+processor.Name(),
			processor,
			5*time.Second, // 5 second timeout
		)
	}

	// RetryWrapper adds retry logic for transient failures.
	// Added: Sprint 11 - Handle transient errors.
	RetryWrapper = func(processor pipz.Chainable[SupportQuery]) pipz.Chainable[SupportQuery] {
		return pipz.NewRetry(
			"retry_"+processor.Name(),
			processor,
			2, // 2 retries
		)
	}
)

// ============================================================================.
// PIPELINE EVOLUTION.
// ============================================================================.
// The journey from "just make it work" to production-grade AI support.

// SupportPipeline is our main pipeline that evolves through sprints.
var SupportPipeline = pipz.NewSequence[SupportQuery](
	PipelineMain,
	ValidateQuery,
	CallGPT4,
	FormatResponse,
)

// ResetPipeline resets to MVP state.
func ResetPipeline() {
	SupportPipeline.Clear()
	SupportPipeline.Register(
		ValidateQuery,
		CallGPT4,
		FormatResponse,
	)
}

// EnableCostOptimization adds intelligent routing to save money.
// Sprint 3: "We can't afford GPT-4 for everything!".
func EnableCostOptimization() {
	// Add classification before AI calls.
	SupportPipeline.Clear()
	SupportPipeline.Register(
		ValidateQuery,
		ClassifyQuery,
		ExtractOrderID,
		// Route based on query type.
		pipz.NewSwitch[SupportQuery](
			ConnectorRouteByType,
			func(_ context.Context, q SupportQuery) string {
				if q.QueryType == QueryTypeOrderStatus {
					return "simple"
				}
				return "complex"
			},
		).
			AddRoute("simple", SimpleQueryPipeline).
			AddRoute("complex", ComplexQueryPipeline),
	)
}

// EnableProviderFallback adds multi-provider resilience.
// Sprint 5: "What happens when OpenAI goes down?".
func EnableProviderFallback() {
	// Replace direct AI calls with fallback chains.
	SupportPipeline.Clear()
	SupportPipeline.Register(
		ValidateQuery,
		ClassifyQuery,
		ExtractOrderID,
		pipz.NewSwitch[SupportQuery](
			ConnectorRouteByType,
			func(_ context.Context, q SupportQuery) string {
				if q.QueryType == QueryTypeOrderStatus {
					return "simple"
				}
				return "complex"
			},
		).
			AddRoute("simple", pipz.NewSequence[SupportQuery](
				"simple_with_fallback",
				CheckCache,
				pipz.Apply("cache_or_fallback", func(ctx context.Context, q SupportQuery) (SupportQuery, error) {
					if q.CacheHit {
						return q, nil
					}
					// Try providers with fallback.
					result, err := GPT35Service.Respond(ctx, q)
					if err != nil {
						// Try Claude.
						result, err = ClaudeService.Respond(ctx, q)
						if err != nil {
							// Try local model as last resort.
							result, err = LocalModelService.Respond(ctx, q)
							if err != nil {
								return q, err
							}
						}
					}
					return result, nil
				}),
				UpdateCache,
				FormatResponse,
			)).
			AddRoute("complex", ComplexQueryWithFallback),
	)
}

// EnableSpeedOptimization adds racing for faster responses.
// Sprint 7: "Our competitors are faster!".
func EnableSpeedOptimization() {
	// Add sentiment analysis and urgency routing.
	SupportPipeline.Clear()
	SupportPipeline.Register(
		ValidateQuery,
		ClassificationPipeline, // Full analysis
		RouteByUrgency,         // Smart routing with race for urgent
	)
}

// EnablePriorityRouting adds sentiment-based priority lanes.
// Sprint 9: "Angry customers need special handling".
func EnablePriorityRouting() {
	// Already implemented in Sprint 7's RouteByUrgency.
	// This sprint was about refining the routing logic.
	EnableSpeedOptimization()
}

// EnableProductionFeatures adds all production requirements.
// Sprint 11: "Make it bulletproof!".
func EnableProductionFeatures() {
	// Full production pipeline with all features.
	ProductionPipeline := pipz.NewHandle[SupportQuery](
		"support_with_recovery",
		pipz.NewSequence[SupportQuery](
			PipelineMain,
			ValidateQuery,
			ClassificationPipeline,
			RouteByUrgency,
			CheckSafety,
			LogMetrics,
		),
		ErrorRecovery,
	)

	// Replace the SupportPipeline with the wrapped version.
	SupportPipeline.Clear()
	SupportPipeline.Register(ProductionPipeline)
}

// ProcessQuery handles a support query through the pipeline.
func ProcessQuery(ctx context.Context, query SupportQuery) (*SupportQuery, error) {
	// Set defaults.
	if query.ID == "" {
		query.ID = fmt.Sprintf("Q-%d", time.Now().UnixNano())
	}
	if query.Timestamp.IsZero() {
		query.Timestamp = time.Now()
	}
	if query.ProcessingLog == nil {
		query.ProcessingLog = make([]string, 0)
	}

	// Process through pipeline.
	result, err := SupportPipeline.Process(ctx, query)
	if err != nil {
		// Even on error, we might have a fallback response.
		if result.Response != "" {
			return &result, nil
		}
		return nil, err
	}

	return &result, nil
}
