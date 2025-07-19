package moderation

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func BenchmarkAnalyzeText(b *testing.B) {
	scenarios := []struct {
		name string
		text string
	}{
		{"Short_Clean", "This is a nice comment"},
		{"Short_Profanity", "This badword1 text"},
		{"Medium_Mixed", "CLICK HERE to WIN! This stupid spam contains badword2 and more!!!"},
		{"Long_Clean", strings.Repeat("This is normal text. ", 50)},
		{"Long_Toxic", strings.Repeat("I hate this stupid thing! ", 20)},
	}

	ctx := context.Background()

	for _, s := range scenarios {
		b.Run(s.name, func(b *testing.B) {
			content := Content{ID: "bench", Text: s.text}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				AnalyzeText(ctx, content)
			}
		})
	}
}

func BenchmarkImageAnalysis(b *testing.B) {
	analyzer := NewMockImageAnalyzer()
	ctx := context.Background()

	scenarios := []struct {
		name      string
		imageData []byte
	}{
		{"Small_Image", make([]byte, 1024)},      // 1KB
		{"Medium_Image", make([]byte, 100*1024)}, // 100KB
		{"Large_Image", make([]byte, 1024*1024)}, // 1MB
	}

	for _, s := range scenarios {
		b.Run(s.name, func(b *testing.B) {
			// Fill with data
			for i := range s.imageData {
				s.imageData[i] = byte(i % 256)
			}

			content := Content{
				ID:        "bench",
				ImageData: s.imageData,
			}

			// Warm up cache
			analyzer.Analyze(ctx, content)

			b.Run("Cached", func(b *testing.B) {
				b.ResetTimer()
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					analyzer.Analyze(ctx, content)
				}
			})

			b.Run("Uncached", func(b *testing.B) {
				b.ResetTimer()
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					// Unique content each time
					uniqueContent := content
					uniqueContent.ImageData = append(s.imageData, byte(i))
					analyzer.Analyze(ctx, uniqueContent)
				}
			})
		})
	}
}

func BenchmarkModerationPipeline(b *testing.B) {
	analyzer := NewMockImageAnalyzer()
	pipeline := CreateModerationPipeline(analyzer, DefaultThresholds)
	ctx := context.Background()

	scenarios := []struct {
		name    string
		content Content
	}{
		{
			name: "Text_Only_Clean",
			content: Content{
				ID:     "bench",
				Text:   "This is a perfectly normal comment about the weather",
				Author: "user",
			},
		},
		{
			name: "Text_Only_Toxic",
			content: Content{
				ID:     "bench",
				Text:   "You stupid badword1! I hate everything about this!!!",
				Author: "troll",
			},
		},
		{
			name: "Image_Only",
			content: Content{
				ID:        "bench",
				ImageData: []byte("test image data"),
				Author:    "user",
			},
		},
		{
			name: "Mixed_Content",
			content: Content{
				ID:        "bench",
				Text:      "Check out this image",
				ImageData: []byte("test image data"),
				Author:    "user",
			},
		},
	}

	for _, s := range scenarios {
		b.Run(s.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				pipeline.Process(ctx, s.content)
			}
		})
	}
}

func BenchmarkBatchProcessing(b *testing.B) {
	analyzer := NewMockImageAnalyzer()
	batchPipeline := CreateBatchModerationPipeline(analyzer, DefaultThresholds)
	ctx := context.Background()

	batchSizes := []int{1, 10, 50, 100}

	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("Batch_%d", size), func(b *testing.B) {
			// Create batch
			batch := make([]Content, size)
			for i := range batch {
				batch[i] = Content{
					ID:     fmt.Sprintf("item-%d", i),
					Text:   fmt.Sprintf("Content item %d with some text", i),
					Author: fmt.Sprintf("user%d", i),
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				batchPipeline.Process(ctx, batch)
			}
		})
	}
}

func BenchmarkDecisionMaking(b *testing.B) {
	decider := MakeDecision(DefaultThresholds)
	ctx := context.Background()

	scores := []float64{0.1, 0.3, 0.5, 0.8, 0.95}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		content := Content{
			ID:           "bench",
			OverallScore: scores[i%len(scores)],
		}
		decider(ctx, content)
	}
}

func BenchmarkReviewQueue(b *testing.B) {
	queue := NewReviewQueue()

	b.Run("Add", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			content := Content{
				ID:       fmt.Sprintf("item-%d", i),
				Decision: DecisionReview,
			}
			queue.Add(content)
		}
	})

	// Pre-populate for Get benchmark
	for i := 0; i < 1000; i++ {
		queue.Add(Content{ID: fmt.Sprintf("pre-%d", i)})
	}

	b.Run("Get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			queue.Get(fmt.Sprintf("pre-%d", i%1000))
		}
	})

	b.Run("List", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			queue.List()
		}
	})
}

func BenchmarkStats(b *testing.B) {
	// Reset stats
	globalStats = &ModerationStats{
		ByDecision: make(map[Decision]int64),
		ByType:     make(map[ContentType]int64),
	}

	contents := []Content{
		{Type: TypeText, Decision: DecisionApprove, OverallScore: 0.1},
		{Type: TypeImage, Decision: DecisionReject, OverallScore: 0.9},
		{Type: TypeMixed, Decision: DecisionReview, OverallScore: 0.5},
	}

	b.Run("Update", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			UpdateStats(contents[i%len(contents)])
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			GetStats()
		}
	})
}

func BenchmarkFullWorkflow(b *testing.B) {
	analyzer := NewMockImageAnalyzer()
	queue := NewReviewQueue()
	storages := map[Decision]Storage{
		DecisionApprove:    NewMemoryStorage(),
		DecisionReject:     NewMemoryStorage(),
		DecisionQuarantine: NewMemoryStorage(),
	}

	moderationPipeline := CreateModerationPipeline(analyzer, DefaultThresholds)
	reviewPipeline := CreateReviewPipeline(queue, storages)

	fullPipeline := pipz.Sequential(
		moderationPipeline,
		reviewPipeline,
	)

	ctx := context.Background()

	// Different content types
	contents := []Content{
		{ID: "1", Text: "Clean text", Author: "user1"},
		{ID: "2", Text: "SPAM CLICK HERE!!!", Author: "spammer"},
		{ID: "3", Text: "I hate you stupid badword1", Author: "troll"},
		{ID: "4", ImageData: []byte("image"), Author: "user2"},
		{ID: "5", Text: "Mixed", ImageData: []byte("data"), Author: "user3"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		content := contents[i%len(contents)]
		content.ID = fmt.Sprintf("%s-%d", content.ID, i) // Unique ID
		fullPipeline.Process(ctx, content)
	}
}

func BenchmarkConcurrentModeration(b *testing.B) {
	analyzer := NewMockImageAnalyzer()
	pipeline := CreateModerationPipeline(analyzer, DefaultThresholds)
	ctx := context.Background()

	content := Content{
		ID:     "bench",
		Text:   "This is a test comment with some words",
		Author: "user",
	}

	b.RunParallel(func(pb *testing.PB) {
		localContent := content
		i := 0
		for pb.Next() {
			localContent.ID = fmt.Sprintf("bench-%d", i)
			pipeline.Process(ctx, localContent)
			i++
		}
	})
}

func BenchmarkRegexPatterns(b *testing.B) {
	texts := []string{
		"normal text without any issues",
		"CLICK HERE TO WIN THE LOTTERY!!!",
		"Buy viagra pills casino lottery",
		"I hate you stupid idiot moron",
		"AAAAAAAAAAAAAAA!!!!!!!!!!!!!!",
	}

	b.Run("Spam_Patterns", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			text := texts[i%len(texts)]
			for _, pattern := range spamPatterns {
				pattern.MatchString(text)
			}
		}
	})

	b.Run("Toxic_Patterns", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			text := texts[i%len(texts)]
			for _, pattern := range toxicPatterns {
				pattern.MatchString(text)
			}
		}
	})
}

var globalContent Content // Prevent compiler optimizations

func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("Content_Allocation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			content := Content{
				ID:        fmt.Sprintf("id-%d", i),
				Type:      TypeMixed,
				Text:      "This is some text content",
				ImageURL:  "https://example.com/image.jpg",
				ImageData: make([]byte, 1024),
				Author:    "user123",
				Context:   "comment",
				Timestamp: time.Now(),
				Flags: []Flag{
					{Type: "spam", Severity: 0.5, Details: "spam detected"},
					{Type: "toxic", Severity: 0.3, Details: "toxic language"},
				},
				Decision: DecisionReview,
				Reason:   "Content requires review",
			}
			globalContent = content
		}
	})

	b.Run("Pipeline_Processing", func(b *testing.B) {
		analyzer := NewMockImageAnalyzer()
		pipeline := CreateModerationPipeline(analyzer, DefaultThresholds)
		ctx := context.Background()

		content := Content{
			ID:     "bench",
			Text:   "Sample text for processing",
			Author: "user",
		}

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			result, _ := pipeline.Process(ctx, content)
			globalContent = result
		}
	})
}
