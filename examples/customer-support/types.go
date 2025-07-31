package main

import (
	"fmt"
	"time"
)

// SupportQuery represents a customer support query through our system.
//
//nolint:govet // Field alignment is optimized for readability
type SupportQuery struct {
	// Request fields.
	ID         string
	CustomerID string
	Query      string
	Timestamp  time.Time
	Channel    string // "web", "mobile", "email"

	// Classification fields.
	QueryType QueryType
	Urgency   UrgencyLevel
	Sentiment SentimentScore

	// Response fields.
	Response     string
	ResponseTime time.Duration
	Provider     string
	Cost         float64
	TokensUsed   int

	// Metadata.
	OrderID       string // Extracted from query if present
	CacheHit      bool
	ProcessingLog []string
	Metrics       QueryMetrics
}

// Clone implements Cloner for concurrent processing.
// IMPORTANT: Deep copy all mutable fields to prevent race conditions.
// when the same query is processed by multiple providers in parallel.
// Without cloning, providers could overwrite each other's data!.
func (q SupportQuery) Clone() SupportQuery {
	// Dev: "Learned this the hard way when our race pattern kept mixing up.
	// responses between providers. Always clone mutable fields!"
	log := make([]string, len(q.ProcessingLog))
	copy(log, q.ProcessingLog)
	q.ProcessingLog = log
	return q
}

// QueryType represents the type of support query.
type QueryType int

const (
	QueryTypeUnknown     QueryType = iota
	QueryTypeOrderStatus           // "Where is my order?"
	QueryTypeRefund                // "I want a refund"
	QueryTypeProduct               // "How does X work?"
	QueryTypeTechnical             // "API error 403"
	QueryTypeComplaint             // "Your service is terrible"
)

func (qt QueryType) String() string {
	switch qt {
	case QueryTypeOrderStatus:
		return "order_status"
	case QueryTypeRefund:
		return "refund"
	case QueryTypeProduct:
		return "product"
	case QueryTypeTechnical:
		return "technical"
	case QueryTypeComplaint:
		return "complaint"
	default:
		return "unknown"
	}
}

// UrgencyLevel represents how urgent the query is.
type UrgencyLevel int

const (
	UrgencyNormal UrgencyLevel = iota
	UrgencyLow
	UrgencyHigh
	UrgencyCritical
)

func (u UrgencyLevel) String() string {
	switch u {
	case UrgencyNormal:
		return "normal"
	case UrgencyLow:
		return "low"
	case UrgencyHigh:
		return "high"
	case UrgencyCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// SentimentScore represents customer emotion.
type SentimentScore int

const (
	SentimentNeutral SentimentScore = iota
	SentimentPositive
	SentimentNegative
	SentimentAngry
)

func (s SentimentScore) String() string {
	switch s {
	case SentimentPositive:
		return "positive"
	case SentimentNegative:
		return "negative"
	case SentimentAngry:
		return "angry"
	default:
		return "neutral"
	}
}

// QueryMetrics tracks performance metrics.
type QueryMetrics struct {
	ClassificationTime time.Duration
	AICallTime         time.Duration
	TotalTime          time.Duration
	RetryCount         int
	FallbackUsed       bool
}

// ProcessorNames - All processor and connector names as constants.
const (
	// Query processing.
	ProcessorValidateQuery    = "validate_query"
	ProcessorClassifyQuery    = "classify_query"
	ProcessorExtractOrderID   = "extract_order_id"
	ProcessorAnalyzeSentiment = "analyze_sentiment"
	ProcessorDetermineUrgency = "determine_urgency"

	// AI calls.
	ProcessorCallGPT4       = "call_gpt4"
	ProcessorCallGPT35      = "call_gpt35"
	ProcessorCallClaude     = "call_claude"
	ProcessorCallLocalModel = "call_local_model"

	// Response processing.
	ProcessorFormatResponse  = "format_response"
	ProcessorAddOrderDetails = "add_order_details"
	ProcessorCheckSafety     = "check_safety"

	// Metrics and logging.
	ProcessorLogMetrics    = "log_metrics"
	ProcessorUpdateCache   = "update_cache"
	ProcessorTrackCustomer = "track_customer"

	// Error handling.
	ProcessorLogError         = "log_error"
	ProcessorNotifyOps        = "notify_ops"
	ProcessorFallbackResponse = "fallback_response"

	// Pipeline names.
	PipelineMain           = "main_support_pipeline"
	PipelineClassification = "query_classification"
	PipelineSimpleQuery    = "simple_query_pipeline"
	PipelineComplexQuery   = "complex_query_pipeline"
	PipelineUrgentQuery    = "urgent_query_pipeline"
	PipelineSafetyCheck    = "safety_check_pipeline"

	// Connector names.
	ConnectorAIProvider     = "ai_provider"
	ConnectorGPTFallback    = "gpt_fallback"
	ConnectorMultiProvider  = "multi_provider_fallback"
	ConnectorSpeedRace      = "speed_race"
	ConnectorWithRetry      = "with_retry"
	ConnectorWithTimeout    = "with_timeout"
	ConnectorRouteByType    = "route_by_type"
	ConnectorRouteByUrgency = "route_by_urgency"
)

// MetricsSummary provides a summary of performance metrics.
//
//nolint:govet // Field alignment is optimized for readability
type MetricsSummary struct {
	TotalQueries      int
	AverageResponse   time.Duration
	FastestResponse   time.Duration
	SlowestResponse   time.Duration
	TotalCost         float64
	CostSavings       float64
	CacheHitRate      float64
	FallbackRate      float64
	ProviderBreakdown map[string]int
}

// PerformanceReport generates a performance report.
func (m MetricsSummary) PerformanceReport() string {
	report := "\n=== Performance Metrics ===\n"
	report += fmt.Sprintf("Total Queries: %d\n", m.TotalQueries)
	report += fmt.Sprintf("Average Response: %v\n", m.AverageResponse)
	report += fmt.Sprintf("Fastest Response: %v\n", m.FastestResponse)
	report += fmt.Sprintf("Slowest Response: %v\n", m.SlowestResponse)
	report += fmt.Sprintf("Total Cost: $%.2f\n", m.TotalCost)
	if m.CostSavings > 0 {
		report += fmt.Sprintf("Cost Savings: $%.2f (%.1f%%)\n", m.CostSavings, (m.CostSavings/(m.TotalCost+m.CostSavings))*100)
	}
	report += fmt.Sprintf("Cache Hit Rate: %.1f%%\n", m.CacheHitRate*100)
	if m.FallbackRate > 0 {
		report += fmt.Sprintf("Fallback Rate: %.1f%%\n", m.FallbackRate*100)
	}
	if len(m.ProviderBreakdown) > 0 {
		report += "\nProvider Usage:\n"
		for provider, count := range m.ProviderBreakdown {
			percentage := float64(count) / float64(m.TotalQueries) * 100
			report += fmt.Sprintf("  %s: %d (%.1f%%)\n", provider, count, percentage)
		}
	}
	return report
}
