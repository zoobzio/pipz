package main

import (
	"fmt"
	"strings"

	"pipz"
)

// Types for composability demo
type Document struct {
	ID      string
	Title   string
	Content string
	Author  string
	Tags    []string
	Score   float64
}

// Individual processors
func validateDocument(d Document) error {
	if d.ID == "" || d.Title == "" || d.Content == "" {
		return fmt.Errorf("missing required fields")
	}
	if len(d.Content) < 10 {
		return fmt.Errorf("content too short")
	}
	return nil
}

func normalizeText(d Document) Document {
	d.Title = strings.TrimSpace(d.Title)
	d.Content = strings.TrimSpace(d.Content)
	// Title case the title
	words := strings.Fields(strings.ToLower(d.Title))
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	d.Title = strings.Join(words, " ")
	return d
}

func extractTags(d Document) Document {
	// Simple tag extraction from content
	words := strings.Fields(strings.ToLower(d.Content))
	tagMap := make(map[string]bool)
	
	// Extract hashtags
	for _, word := range words {
		if strings.HasPrefix(word, "#") && len(word) > 1 {
			tagMap[strings.TrimPrefix(word, "#")] = true
		}
	}
	
	// Convert to slice
	d.Tags = make([]string, 0, len(tagMap))
	for tag := range tagMap {
		d.Tags = append(d.Tags, tag)
	}
	
	return d
}

func scoreDocument(d Document) Document {
	// Simple scoring based on content
	score := 0.0
	
	// Length score
	if len(d.Content) > 100 {
		score += 2.0
	}
	if len(d.Content) > 500 {
		score += 3.0
	}
	
	// Tag score
	score += float64(len(d.Tags)) * 0.5
	
	// Title score
	if len(d.Title) > 10 && len(d.Title) < 60 {
		score += 1.0
	}
	
	d.Score = score
	return d
}

func runComposabilityDemo() {
	section("PIPELINE COMPOSABILITY")
	
	info("Use Case: Document Processing System")
	info("• Build modular, reusable pipelines")
	info("• Compose pipelines from smaller units")
	info("• Chain multiple pipelines together")
	info("• Mix validation, transformation, and enrichment")

	// Create individual pipelines
	validationPipeline := pipz.NewContract[Document]()
	validationPipeline.Register(pipz.Validate(validateDocument))

	normalizationPipeline := pipz.NewContract[Document]()
	normalizationPipeline.Register(pipz.Transform(normalizeText))

	enrichmentPipeline := pipz.NewContract[Document]()
	enrichmentPipeline.Register(
		pipz.Transform(extractTags),
		pipz.Transform(scoreDocument),
	)

	// Compose them together
	chain := pipz.NewChain[Document]()
	chain.Add(
		validationPipeline,
		normalizationPipeline,
		enrichmentPipeline,
	)

	code("go", `// Create modular pipelines
validation := pipz.NewContract[Document]()
validation.Register(pipz.Validate(validateDocument))

normalization := pipz.NewContract[Document]()
normalization.Register(pipz.Transform(normalizeText))

enrichment := pipz.NewContract[Document]()
enrichment.Register(
    pipz.Transform(extractTags),
    pipz.Transform(scoreDocument),
)

// Compose into processing chain
chain := pipz.NewChain[Document]()
chain.Add(validation, normalization, enrichment)`)

	// Test Case 1: Valid document
	fmt.Println("\n📄 Test Case 1: Complete Document")
	doc1 := Document{
		ID:      "DOC-001",
		Title:   "  introduction to PIPELINES  ",
		Content: "This is a comprehensive guide to building #scalable and #efficient processing pipelines. " +
			"We'll explore #composability patterns and best practices for #golang development.",
		Author: "Jane Developer",
	}

	result, err := chain.Process(doc1)
	if err != nil {
		showError(fmt.Sprintf("Processing failed: %v", err))
	} else {
		success("Document processed successfully")
		fmt.Printf("   Title: '%s' (normalized)\n", result.Title)
		fmt.Printf("   Tags: %v\n", result.Tags)
		fmt.Printf("   Score: %.1f\n", result.Score)
	}

	// Test Case 2: Create custom pipeline
	fmt.Println("\n📄 Test Case 2: Custom Pipeline Composition")
	
	// Quality check pipeline
	qualityPipeline := pipz.NewContract[Document]()
	qualityPipeline.Register(
		pipz.Validate(validateDocument),
		pipz.Mutate(
			func(d Document) Document {
				d.Title = "[PREMIUM] " + d.Title
				d.Score *= 1.5
				return d
			},
			func(d Document) bool {
				return d.Score > 5.0 // Premium content
			},
		),
	)

	// Process premium content
	premiumChain := pipz.NewChain[Document]()
	premiumChain.Add(
		validationPipeline,
		normalizationPipeline,
		enrichmentPipeline,
		qualityPipeline,
	)

	doc2 := Document{
		ID:      "DOC-002",
		Title:   "advanced pipeline techniques",
		Content: strings.Repeat("Advanced content about #pipelines #architecture #performance #optimization #scalability ", 20),
		Author:  "Expert Author",
	}

	result, err = premiumChain.Process(doc2)
	if err != nil {
		showError(fmt.Sprintf("Processing failed: %v", err))
	} else {
		success("Premium document processed")
		fmt.Printf("   Title: '%s'\n", result.Title)
		fmt.Printf("   Score: %.1f (premium boost applied)\n", result.Score)
	}

	// Test Case 3: Partial pipeline
	fmt.Println("\n📄 Test Case 3: Partial Pipeline (Skip Validation)")
	
	// Just enrichment
	doc3 := Document{
		Title:   "quick note",
		Content: "A brief note with #todo and #reminder tags",
	}
	
	// Skip validation for drafts
	draftChain := pipz.NewChain[Document]()
	draftChain.Add(normalizationPipeline, enrichmentPipeline)
	
	result, err = draftChain.Process(doc3)
	if err != nil {
		showError(fmt.Sprintf("Processing failed: %v", err))
	} else {
		success("Draft processed without validation")
		fmt.Printf("   Title: '%s'\n", result.Title)
		fmt.Printf("   Tags: %v\n", result.Tags)
	}

	fmt.Println("\n🔍 Composability Patterns:")
	info("• Build small, focused pipelines")
	info("• Compose complex workflows from simple parts")
	info("• Reuse pipelines across different contexts")
	info("• Mix and match based on requirements")
}