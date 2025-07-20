// Package moderation demonstrates using pipz for content moderation pipelines
// that can analyze text and images, score content, and route to appropriate actions.
package moderation

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zoobzio/pipz"
)

// Content represents content to be moderated
type Content struct {
	ID        string
	Type      ContentType
	Text      string
	ImageURL  string
	ImageData []byte
	Author    string
	Context   string // e.g., "comment", "profile", "message"
	Timestamp time.Time

	// Analysis results
	TextScore    float64
	ImageScore   float64
	OverallScore float64
	Flags        []Flag
	Decision     Decision
	Reason       string

	// Processing metadata
	AnalyzedAt time.Time
	ReviewedAt time.Time
	ReviewedBy string
}

type ContentType string

const (
	TypeText  ContentType = "text"
	TypeImage ContentType = "image"
	TypeMixed ContentType = "mixed"
)

type Decision string

const (
	DecisionApprove    Decision = "approve"
	DecisionReview     Decision = "review"
	DecisionQuarantine Decision = "quarantine"
	DecisionReject     Decision = "reject"
)

type Flag struct {
	Type     string
	Severity float64
	Details  string
}

// Thresholds for content scoring
type Thresholds struct {
	AutoApprove float64 // < this score = auto-approve
	Review      float64 // >= this score = human review
	Quarantine  float64 // >= this score = quarantine
	AutoReject  float64 // >= this score = auto-reject
}

var DefaultThresholds = Thresholds{
	AutoApprove: 0.2,
	Review:      0.4,
	Quarantine:  0.7,
	AutoReject:  0.9,
}

// Text Analysis

var (
	// Profanity patterns (simplified for demo)
	profanityPatterns = []string{
		"badword1", "badword2", "badword3",
	}

	// Spam patterns
	spamPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(viagra|pills|casino|lottery)\b`),
		regexp.MustCompile(`(?i)\b(click here|buy now|limited time)\b`),
		regexp.MustCompile(`\b[A-Z]{5,}\b`), // Excessive caps
		regexp.MustCompile(`[!?.]{5,}`),     // Repeated punctuation
	}

	// Toxic patterns
	toxicPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(hate|kill|die)\b`),
		regexp.MustCompile(`(?i)\b(stupid|idiot|moron)\b`),
	}
)

// AnalyzeText performs text content analysis
func AnalyzeText(_ context.Context, content Content) (Content, error) {
	if content.Text == "" {
		return content, nil
	}

	score := 0.0
	flags := []Flag{}

	text := strings.ToLower(content.Text)

	// Check profanity
	for _, word := range profanityPatterns {
		if strings.Contains(text, word) {
			score += 0.3
			flags = append(flags, Flag{
				Type:     "profanity",
				Severity: 0.3,
				Details:  fmt.Sprintf("Contains profanity: %s", word),
			})
		}
	}

	// Check spam patterns
	spamScore := 0.0
	for _, pattern := range spamPatterns {
		if pattern.MatchString(content.Text) {
			spamScore += 0.2
			flags = append(flags, Flag{
				Type:     "spam",
				Severity: 0.2,
				Details:  fmt.Sprintf("Matches spam pattern: %s", pattern.String()),
			})
		}
	}
	if spamScore > 0 {
		score += math.Min(spamScore, 0.5) // Cap spam contribution
	}

	// Check toxic patterns
	for _, pattern := range toxicPatterns {
		if pattern.MatchString(text) {
			score += 0.4
			flags = append(flags, Flag{
				Type:     "toxic",
				Severity: 0.4,
				Details:  fmt.Sprintf("Contains toxic language"),
			})
		}
	}

	// Check for all caps (shouting)
	if len(content.Text) > 10 {
		upperCount := 0
		for _, r := range content.Text {
			if r >= 'A' && r <= 'Z' {
				upperCount++
			}
		}
		capsRatio := float64(upperCount) / float64(len(content.Text))
		if capsRatio > 0.7 {
			score += 0.1
			flags = append(flags, Flag{
				Type:     "formatting",
				Severity: 0.1,
				Details:  "Excessive capital letters",
			})
		}
	}

	// Normalize score to 0-1 range
	content.TextScore = math.Min(score, 1.0)
	content.Flags = append(content.Flags, flags...)

	return content, nil
}

// Image Analysis

// MockImageAnalyzer simulates image analysis
type MockImageAnalyzer struct {
	cache sync.Map
}

func NewMockImageAnalyzer() *MockImageAnalyzer {
	return &MockImageAnalyzer{}
}

func (m *MockImageAnalyzer) Analyze(_ context.Context, content Content) (Content, error) {
	if content.ImageData == nil && content.ImageURL == "" {
		return content, nil
	}

	// Generate deterministic score based on image hash
	var hash string
	if content.ImageData != nil {
		h := md5.Sum(content.ImageData)
		hash = hex.EncodeToString(h[:])
	} else {
		hash = content.ImageURL
	}

	// Check cache
	if cached, ok := m.cache.Load(hash); ok {
		result := cached.(imageAnalysisResult)
		content.ImageScore = result.score
		content.Flags = append(content.Flags, result.flags...)
		return content, nil
	}

	// Simulate analysis
	time.Sleep(10 * time.Millisecond) // Simulate API call

	// Mock scoring based on hash
	hashBytes := []byte(hash)
	score := float64(hashBytes[0]) / 255.0

	flags := []Flag{}
	if score > 0.5 {
		flags = append(flags, Flag{
			Type:     "inappropriate_image",
			Severity: score,
			Details:  "Image contains potentially inappropriate content",
		})
	}

	// Cache result
	m.cache.Store(hash, imageAnalysisResult{score: score, flags: flags})

	content.ImageScore = score
	content.Flags = append(content.Flags, flags...)

	return content, nil
}

type imageAnalysisResult struct {
	score float64
	flags []Flag
}

// Combined Analysis

// CombineScores calculates overall score from text and image scores
func CombineScores(_ context.Context, content Content) (Content, error) {
	switch content.Type {
	case TypeText:
		content.OverallScore = content.TextScore
	case TypeImage:
		content.OverallScore = content.ImageScore
	case TypeMixed:
		// Weighted average with higher weight on max score
		maxScore := math.Max(content.TextScore, content.ImageScore)
		avgScore := (content.TextScore + content.ImageScore) / 2
		content.OverallScore = 0.7*maxScore + 0.3*avgScore
	}

	content.AnalyzedAt = time.Now()
	return content, nil
}

// Decision Making

// MakeDecision determines the moderation decision based on score and thresholds
func MakeDecision(thresholds Thresholds) func(context.Context, Content) (Content, error) {
	return func(_ context.Context, content Content) (Content, error) {
		score := content.OverallScore

		switch {
		case score >= thresholds.AutoReject:
			content.Decision = DecisionReject
			content.Reason = fmt.Sprintf("Content score %.2f exceeds auto-reject threshold", score)

		case score >= thresholds.Quarantine:
			content.Decision = DecisionQuarantine
			content.Reason = fmt.Sprintf("Content score %.2f requires quarantine", score)

		case score >= thresholds.Review:
			content.Decision = DecisionReview
			content.Reason = fmt.Sprintf("Content score %.2f requires human review", score)

		default:
			content.Decision = DecisionApprove
			content.Reason = fmt.Sprintf("Content score %.2f below review threshold", score)
		}

		return content, nil
	}
}

// Human Review

// ReviewQueue manages content pending human review
type ReviewQueue struct {
	items sync.Map
	mu    sync.RWMutex
}

func NewReviewQueue() *ReviewQueue {
	return &ReviewQueue{}
}

func (q *ReviewQueue) Add(content Content) {
	q.items.Store(content.ID, content)
}

func (q *ReviewQueue) Get(id string) (Content, bool) {
	if item, ok := q.items.Load(id); ok {
		return item.(Content), true
	}
	return Content{}, false
}

func (q *ReviewQueue) Review(id string, decision Decision, reviewer string) error {
	item, ok := q.items.Load(id)
	if !ok {
		return errors.New("content not found in review queue")
	}

	content := item.(Content)
	content.Decision = decision
	content.ReviewedAt = time.Now()
	content.ReviewedBy = reviewer

	q.items.Delete(id)
	return nil
}

func (q *ReviewQueue) List() []Content {
	var items []Content
	q.items.Range(func(key, value interface{}) bool {
		items = append(items, value.(Content))
		return true
	})
	return items
}

// Actions

// LogDecision records moderation decisions
func LogDecision(_ context.Context, content Content) error {
	// Skip logging in benchmarks (when content ID is "bench")
	if content.ID == "bench" {
		return nil
	}

	fmt.Printf("[MODERATION] Content %s: %s (score: %.2f)\n",
		content.ID, content.Decision, content.OverallScore)

	if len(content.Flags) > 0 {
		fmt.Printf("  Flags:\n")
		for _, flag := range content.Flags {
			fmt.Printf("    - %s (severity: %.2f): %s\n",
				flag.Type, flag.Severity, flag.Details)
		}
	}

	return nil
}

// NotifyAuthor sends notification to content author
func NotifyAuthor(_ context.Context, content Content) error {
	// Skip logging in benchmarks
	if content.ID == "bench" {
		return nil
	}

	if content.Decision == DecisionReject || content.Decision == DecisionQuarantine {
		fmt.Printf("[NOTIFICATION] Dear %s, your content has been %s. Reason: %s\n",
			content.Author, content.Decision, content.Reason)
	}
	return nil
}

// StoreContent saves content to appropriate storage
func StoreContent(storages map[Decision]Storage) func(context.Context, Content) error {
	return func(_ context.Context, content Content) error {
		storage, ok := storages[content.Decision]
		if !ok {
			return fmt.Errorf("no storage configured for decision: %s", content.Decision)
		}

		return storage.Store(content)
	}
}

type Storage interface {
	Store(Content) error
}

type MemoryStorage struct {
	items sync.Map
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

func (s *MemoryStorage) Store(content Content) error {
	s.items.Store(content.ID, content)
	return nil
}

func (s *MemoryStorage) Get(id string) (Content, bool) {
	if item, ok := s.items.Load(id); ok {
		return item.(Content), true
	}
	return Content{}, false
}

// Pipeline Builders

// CreateModerationPipeline creates the main content moderation pipeline
func CreateModerationPipeline(analyzer *MockImageAnalyzer, thresholds Thresholds) pipz.Chainable[Content] {
	// Determine content type
	determineType := pipz.Apply("determine_type", func(_ context.Context, content Content) (Content, error) {
		hasText := content.Text != ""
		hasImage := content.ImageData != nil || content.ImageURL != ""

		switch {
		case hasText && hasImage:
			content.Type = TypeMixed
		case hasImage:
			content.Type = TypeImage
		case hasText:
			content.Type = TypeText
		default:
			return content, errors.New("no content to moderate")
		}

		return content, nil
	})

	// Parallel analysis for mixed content
	analyzeContent := pipz.Apply("analyze_content", func(ctx context.Context, content Content) (Content, error) {
		if content.Type == TypeMixed {
			// Run text and image analysis in parallel
			var textErr, imageErr error
			var textResult, imageResult Content
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				textResult, textErr = AnalyzeText(ctx, content)
			}()

			go func() {
				defer wg.Done()
				imageResult, imageErr = analyzer.Analyze(ctx, content)
			}()

			wg.Wait()

			if textErr != nil {
				return content, textErr
			}
			if imageErr != nil {
				return content, imageErr
			}

			// Merge results
			content.TextScore = textResult.TextScore
			content.Flags = append(content.Flags, textResult.Flags...)
			content.ImageScore = imageResult.ImageScore
			content.Flags = append(content.Flags, imageResult.Flags...)
		} else if content.Type == TypeText {
			return AnalyzeText(ctx, content)
		} else {
			return analyzer.Analyze(ctx, content)
		}

		return content, nil
	})

	return pipz.Sequential(
		determineType,
		analyzeContent,
		pipz.Apply("combine_scores", CombineScores),
		pipz.Apply("make_decision", MakeDecision(thresholds)),
		pipz.Effect("log_decision", LogDecision),
		pipz.Effect("notify_author", NotifyAuthor),
	)
}

// CreateBatchModerationPipeline creates a pipeline for batch processing
func CreateBatchModerationPipeline(analyzer *MockImageAnalyzer, thresholds Thresholds) pipz.Chainable[[]Content] {
	singlePipeline := CreateModerationPipeline(analyzer, thresholds)

	return pipz.Apply("batch_moderate", func(ctx context.Context, batch []Content) ([]Content, error) {
		results := make([]Content, len(batch))
		errors := make([]error, len(batch))

		var wg sync.WaitGroup
		wg.Add(len(batch))

		for i := range batch {
			go func(idx int) {
				defer wg.Done()
				results[idx], errors[idx] = singlePipeline.Process(ctx, batch[idx])
			}(i)
		}

		wg.Wait()

		// Check for errors
		for i, err := range errors {
			if err != nil {
				return nil, fmt.Errorf("error processing content %s: %w", batch[i].ID, err)
			}
		}

		return results, nil
	})
}

// CreateReviewPipeline creates a pipeline for human review workflow
func CreateReviewPipeline(queue *ReviewQueue, storages map[Decision]Storage) pipz.Chainable[Content] {
	// Route based on decision
	routeByDecision := func(ctx context.Context, content Content) string {
		return string(content.Decision)
	}

	handlers := map[string]pipz.Chainable[Content]{
		string(DecisionApprove): pipz.Sequential(
			pipz.Effect("store_approved", StoreContent(storages)),
		),
		string(DecisionReview): pipz.Apply("queue_for_review", func(ctx context.Context, content Content) (Content, error) {
			queue.Add(content)
			fmt.Printf("[REVIEW] Content %s queued for human review\n", content.ID)
			return content, nil
		}),
		string(DecisionQuarantine): pipz.Sequential(
			pipz.Effect("store_quarantined", StoreContent(storages)),
			pipz.Apply("queue_for_review", func(ctx context.Context, content Content) (Content, error) {
				queue.Add(content)
				return content, nil
			}),
		),
		string(DecisionReject): pipz.Effect("store_rejected", StoreContent(storages)),
	}

	return pipz.Switch(routeByDecision, handlers)
}

// Statistics

type ModerationStats struct {
	Total        int64
	ByDecision   map[Decision]int64
	ByType       map[ContentType]int64
	AverageScore float64
	mu           sync.RWMutex
}

var globalStats = &ModerationStats{
	ByDecision: make(map[Decision]int64),
	ByType:     make(map[ContentType]int64),
}

func UpdateStats(content Content) {
	globalStats.mu.Lock()
	defer globalStats.mu.Unlock()

	globalStats.Total++
	globalStats.ByDecision[content.Decision]++
	globalStats.ByType[content.Type]++

	// Update average score
	prevAvg := globalStats.AverageScore
	globalStats.AverageScore = (prevAvg*float64(globalStats.Total-1) + content.OverallScore) / float64(globalStats.Total)
}

func GetStats() ModerationStats {
	globalStats.mu.RLock()
	defer globalStats.mu.RUnlock()

	return ModerationStats{
		Total:        globalStats.Total,
		ByDecision:   copyDecisionMap(globalStats.ByDecision),
		ByType:       copyTypeMap(globalStats.ByType),
		AverageScore: globalStats.AverageScore,
	}
}

func copyDecisionMap(m map[Decision]int64) map[Decision]int64 {
	copy := make(map[Decision]int64)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

func copyTypeMap(m map[ContentType]int64) map[ContentType]int64 {
	copy := make(map[ContentType]int64)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}
