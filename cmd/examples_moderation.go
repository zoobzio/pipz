package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	"github.com/zoobzio/pipz/examples/moderation"
)

// ModerationExample implements the Example interface for content moderation
type ModerationExample struct{}

func (m *ModerationExample) Name() string {
	return "moderation"
}

func (m *ModerationExample) Description() string {
	return "Content moderation with text analysis, image scanning, and automated decision making"
}

func (m *ModerationExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ CONTENT MODERATION EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Multi-modal content analysis (text + images)")
	fmt.Println("• Parallel processing for mixed content types")
	fmt.Println("• Configurable scoring thresholds for decisions")
	fmt.Println("• Automated routing to approval, review, or rejection")
	fmt.Println("• Human review queue management")
	fmt.Println("• Batch processing for high-volume content")

	waitForEnter()

	// Show the problem
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("Content moderation involves multiple detection systems:")
	fmt.Println(colorGray + `
func moderateContent(content Content) error {
    // Text analysis
    textScore := 0.0
    if containsProfanity(content.Text) {
        textScore += 0.4
    }
    if isSpam(content.Text) {
        textScore += 0.3
    }
    if isToxic(content.Text) {
        textScore += 0.5
    }
    
    // Image analysis
    imageScore := 0.0
    if content.ImageURL != "" {
        result, err := imageAnalysisAPI.Analyze(content.ImageURL)
        if err != nil {
            log.Printf("Image analysis failed: %v", err)
            // Continue without image score?
        } else {
            imageScore = result.Score
        }
    }
    
    // Combine scores
    overallScore := math.Max(textScore, imageScore)
    
    // Make decision
    if overallScore > 0.8 {
        return rejectContent(content)
    } else if overallScore > 0.5 {
        return queueForReview(content)
    } else if overallScore > 0.3 {
        return quarantineContent(content)
    }
    
    return approveContent(content)
}` + colorReset)

	waitForEnter()

	// Show the solution
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("Use the real moderation example with proper pipeline composition:")
	fmt.Println(colorGray + `
// Import the real moderation package
import "github.com/zoobzio/pipz/examples/moderation"

// Create analyzer and configure thresholds
analyzer := moderation.NewMockImageAnalyzer()
thresholds := moderation.DefaultThresholds

// Build comprehensive moderation pipeline
moderationPipeline := moderation.CreateModerationPipeline(analyzer, thresholds)

// Process content with automatic routing
content := moderation.Content{
    ID:        "content_001",
    Type:      moderation.TypeMixed,
    Text:      "User comment text",
    ImageURL:  "https://example.com/image.jpg",
    Author:    "user123",
    Timestamp: time.Now(),
}

result, err := moderationPipeline.Process(ctx, content)` + colorReset)

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's moderate some content!" + colorReset)
	return m.runInteractive(ctx)
}

func (m *ModerationExample) runInteractive(ctx context.Context) error {
	// Initialize services
	analyzer := moderation.NewMockImageAnalyzer()
	thresholds := moderation.DefaultThresholds
	reviewQueue := moderation.NewReviewQueue()

	// Create storage for different decisions
	storages := map[moderation.Decision]moderation.Storage{
		moderation.DecisionApprove:    moderation.NewMemoryStorage(),
		moderation.DecisionReview:     moderation.NewMemoryStorage(),
		moderation.DecisionQuarantine: moderation.NewMemoryStorage(),
		moderation.DecisionReject:     moderation.NewMemoryStorage(),
	}

	// Create pipelines
	moderationPipeline := moderation.CreateModerationPipeline(analyzer, thresholds)
	reviewPipeline := moderation.CreateReviewPipeline(reviewQueue, storages)
	batchPipeline := moderation.CreateBatchModerationPipeline(analyzer, thresholds)

	// Test scenarios
	scenarios := []struct {
		name    string
		content moderation.Content
		special string
	}{
		{
			name: "Clean Text Content",
			content: moderation.Content{
				ID:        "content_001",
				Type:      moderation.TypeText,
				Text:      "This is a great product! I really enjoyed using it.",
				Author:    "user_001",
				Context:   "product_review",
				Timestamp: time.Now(),
			},
		},
		{
			name: "Spam Content",
			content: moderation.Content{
				ID:        "content_002",
				Type:      moderation.TypeText,
				Text:      "CLICK HERE NOW!!! LIMITED TIME OFFER!!! BUY VIAGRA PILLS!!!",
				Author:    "spammer_001",
				Context:   "comment",
				Timestamp: time.Now(),
			},
		},
		{
			name: "Toxic Language",
			content: moderation.Content{
				ID:        "content_003",
				Type:      moderation.TypeText,
				Text:      "You're such an idiot! I hate this stupid product!",
				Author:    "angry_user",
				Context:   "product_review",
				Timestamp: time.Now(),
			},
		},
		{
			name: "Mixed Content - Text + Image",
			content: moderation.Content{
				ID:        "content_004",
				Type:      moderation.TypeMixed,
				Text:      "Check out this cool image I found!",
				ImageURL:  "https://example.com/suspicious-image.jpg",
				ImageData: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A}, // Mock PNG header
				Author:    "user_002",
				Context:   "social_post",
				Timestamp: time.Now(),
			},
		},
		{
			name: "Image Only Content",
			content: moderation.Content{
				ID:        "content_005",
				Type:      moderation.TypeImage,
				ImageURL:  "https://example.com/profile-pic.jpg",
				ImageData: []byte{0xFF, 0xD8, 0xFF, 0xE0}, // Mock JPEG header
				Author:    "user_003",
				Context:   "profile_picture",
				Timestamp: time.Now(),
			},
		},
		{
			name:    "Batch Processing Test",
			special: "batch",
		},
	}

	processedCount := 0

	for i, scenario := range scenarios {
		fmt.Printf("\n%s═══ Scenario %d: %s ═══%s\n",
			colorWhite, i+1, scenario.name, colorReset)

		if scenario.special == "batch" {
			// Test batch processing
			fmt.Printf("\n%sProcessing batch of mixed content...%s\n", colorYellow, colorReset)

			batch := []moderation.Content{
				{
					ID:        "batch_001",
					Type:      moderation.TypeText,
					Text:      "Great product!",
					Author:    "user_batch_1",
					Timestamp: time.Now(),
				},
				{
					ID:        "batch_002",
					Type:      moderation.TypeText,
					Text:      "SPAM SPAM SPAM!!!",
					Author:    "spammer_batch",
					Timestamp: time.Now(),
				},
				{
					ID:        "batch_003",
					Type:      moderation.TypeMixed,
					Text:      "Check this out",
					ImageData: []byte{0x89, 0x50, 0x4E, 0x47},
					Author:    "user_batch_2",
					Timestamp: time.Now(),
				},
			}

			fmt.Printf("Batch contains %d items:\n", len(batch))
			for j, item := range batch {
				fmt.Printf("  %d. %s (%s): \"%s\"\n", j+1, item.ID, item.Type, 
					truncateText(item.Text, 30))
			}

			start := time.Now()
			results, err := batchPipeline.Process(ctx, batch)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("\n%s❌ Batch Processing Failed: %s%s\n", colorRed, err.Error(), colorReset)
			} else {
				fmt.Printf("\n%s✅ Batch Processed Successfully%s\n", colorGreen, colorReset)
				fmt.Printf("  Items processed: %d\n", len(results))
				fmt.Printf("  Processing time: %v\n", duration)
				fmt.Printf("  Throughput: %.0f items/sec\n", 
					float64(len(results))/duration.Seconds())

				// Show results summary
				decisionCounts := make(map[moderation.Decision]int)
				for _, result := range results {
					decisionCounts[result.Decision]++
				}

				fmt.Printf("\n  Decision Breakdown:\n")
				for decision, count := range decisionCounts {
					fmt.Printf("    %s: %d\n", decision, count)
				}
			}
		} else {
			// Single content processing
			fmt.Printf("\nContent Details:\n")
			fmt.Printf("  ID: %s\n", scenario.content.ID)
			fmt.Printf("  Type: %s\n", scenario.content.Type)
			fmt.Printf("  Author: %s\n", scenario.content.Author)
			fmt.Printf("  Context: %s\n", scenario.content.Context)

			if scenario.content.Text != "" {
				fmt.Printf("  Text: \"%s\"\n", truncateText(scenario.content.Text, 50))
			}

			if scenario.content.ImageURL != "" {
				fmt.Printf("  Image URL: %s\n", scenario.content.ImageURL)
			}

			if scenario.content.ImageData != nil {
				fmt.Printf("  Image Data: %d bytes\n", len(scenario.content.ImageData))
			}

			fmt.Printf("\n%sAnalyzing content...%s\n", colorYellow, colorReset)
			start := time.Now()

			result, err := moderationPipeline.Process(ctx, scenario.content)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("\n%s❌ Moderation Failed%s\n", colorRed, colorReset)
				
				var pipelineErr *pipz.PipelineError[moderation.Content]
				if errors.As(err, &pipelineErr) {
					fmt.Printf("  Failed at: %s%s%s (stage %d)\n",
						colorYellow, pipelineErr.ProcessorName, colorReset,
						pipelineErr.StageIndex)
				}
				
				fmt.Printf("  Error: %s\n", err.Error())
			} else {
				fmt.Printf("\n%s✅ Content Analyzed%s\n", colorGreen, colorReset)
				fmt.Printf("  Decision: %s%s%s\n", getDecisionColor(result.Decision), 
					result.Decision, colorReset)
				fmt.Printf("  Overall Score: %.2f\n", result.OverallScore)
				fmt.Printf("  Reason: %s\n", result.Reason)
				fmt.Printf("  Processing Time: %v\n", duration)

				if result.Type == moderation.TypeMixed || result.Type == moderation.TypeText {
					fmt.Printf("  Text Score: %.2f\n", result.TextScore)
				}

				if result.Type == moderation.TypeMixed || result.Type == moderation.TypeImage {
					fmt.Printf("  Image Score: %.2f\n", result.ImageScore)
				}

				// Show flags
				if len(result.Flags) > 0 {
					fmt.Printf("\n  Detected Issues:\n")
					for _, flag := range result.Flags {
						fmt.Printf("    ⚠️  %s (severity: %.2f): %s\n", 
							flag.Type, flag.Severity, flag.Details)
					}
				}

				// Process through review pipeline
				fmt.Printf("\n%sProcessing through review workflow...%s\n", colorYellow, colorReset)
				_, err = reviewPipeline.Process(ctx, result)
				if err != nil {
					fmt.Printf("  Review processing failed: %s\n", err.Error())
				}

				processedCount++
			}
		}

		if i < len(scenarios)-1 {
			waitForEnter()
		}
	}

	// Show review queue status
	fmt.Printf("\n%s═══ REVIEW QUEUE STATUS ═══%s\n", colorCyan, colorReset)
	queueItems := reviewQueue.List()
	fmt.Printf("\nItems in Review Queue: %d\n", len(queueItems))

	if len(queueItems) > 0 {
		fmt.Printf("\nPending Review:\n")
		for _, item := range queueItems {
			fmt.Printf("  • %s (%s) - Score: %.2f\n", item.ID, item.Decision, item.OverallScore)
		}
	}

	// Show threshold information
	fmt.Printf("\n%s═══ MODERATION THRESHOLDS ═══%s\n", colorCyan, colorReset)
	fmt.Printf("\nConfigured Thresholds:\n")
	fmt.Printf("  Auto-Approve: < %.2f\n", thresholds.AutoApprove)
	fmt.Printf("  Human Review: %.2f - %.2f\n", thresholds.Review, thresholds.Quarantine-0.01)
	fmt.Printf("  Quarantine: %.2f - %.2f\n", thresholds.Quarantine, thresholds.AutoReject-0.01)
	fmt.Printf("  Auto-Reject: ≥ %.2f\n", thresholds.AutoReject)

	// Show best practices
	fmt.Printf("\n%s═══ MODERATION BEST PRACTICES ═══%s\n", colorCyan, colorReset)
	fmt.Printf("\n• Use multiple analysis methods (text + image + metadata)\n")
	fmt.Printf("• Implement configurable thresholds for different content types\n")
	fmt.Printf("• Provide clear feedback to users about policy violations\n")
	fmt.Printf("• Use human review for edge cases and appeals\n")
	fmt.Printf("• Track false positives and adjust thresholds accordingly\n")
	fmt.Printf("• Cache analysis results to avoid re-processing\n")
	fmt.Printf("• Use batch processing for high-volume content streams\n")

	return nil
}

func (m *ModerationExample) Benchmark(b *testing.B) error {
	return nil
}

// Helper functions

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func getDecisionColor(decision moderation.Decision) string {
	switch decision {
	case moderation.DecisionApprove:
		return colorGreen
	case moderation.DecisionReview:
		return colorYellow
	case moderation.DecisionQuarantine:
		return colorYellow
	case moderation.DecisionReject:
		return colorRed
	default:
		return colorWhite
	}
}
