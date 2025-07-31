// Customer Support Chatbot Example.
// ================================.
// This is a DEMONSTRATION of building an AI-powered customer support system.
// using the pipz library. It shows how a simple MVP can evolve into a
// production-grade system through progressive enhancement.
//
// IMPORTANT: This example uses MOCK AI services for demonstration purposes.
// In a real implementation, you would integrate with actual AI providers.
// like OpenAI, Anthropic, Google, etc.
//
// Run with: go run .
// Run specific sprint: go run . -sprint=3

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	var sprint int
	flag.IntVar(&sprint, "sprint", 0, "Run specific sprint (1-11), 0 for full demo")
	flag.Parse()

	fmt.Println("=== Customer Support AI Pipeline Demo ===")
	fmt.Println("Building an AI-powered support system that evolves from MVP to production-grade")
	fmt.Println()

	ctx := context.Background()

	if sprint > 0 {
		// Run specific sprint.
		runSprint(ctx, sprint)
	} else {
		// Run full evolution demo.
		runFullDemo(ctx)
	}
}

func runFullDemo(ctx context.Context) {
	// Sprint 1: MVP.
	fmt.Println("📅 SPRINT 1: MVP - Just Answer Questions!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	ResetPipeline()

	query1 := SupportQuery{
		CustomerID: "CUST-001",
		Query:      "Where is my order #12345?",
		Channel:    "web",
	}

	result1, err := ProcessQuery(ctx, query1)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Response in %v (Cost: $%.4f)\n", result1.ResponseTime, result1.Cost)
		fmt.Printf("Provider: %s\n", result1.Provider)
		fmt.Printf("Response: %.100s...\n", result1.Response)
	}

	// Show the problem.
	fmt.Printf("\n💸 Monthly projection at 10k queries/day: $%.2f\n", result1.Cost*10000*30)
	fmt.Println("\nCFO: 'WHAT?! We're spending HOW MUCH on AI?!'")
	fmt.Println()
	time.Sleep(2 * time.Second)

	// Sprint 3: Cost Optimization.
	fmt.Println("\n📅 SPRINT 3: Cost Crisis - Smart Routing")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Solution: Route simple 'where is order' queries to GPT-3.5")
	EnableCostOptimization()
	ResponseCache.Clear() // Clear cache for demo

	// Test same query with new pipeline.
	result3, err := ProcessQuery(ctx, query1)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	fmt.Printf("✅ Same query now costs: $%.4f (was $%.4f)\n", result3.Cost, result1.Cost)
	fmt.Printf("Provider: %s\n", result3.Provider)
	fmt.Printf("💰 Savings: %.1f%%\n", (1-result3.Cost/result1.Cost)*100)

	// Test cache.
	fmt.Println("\nTesting cache for repeated queries...")
	result3b, err := ProcessQuery(ctx, query1)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	fmt.Printf("✅ Cached response: $%.4f (instant!)\n", result3b.Cost)
	fmt.Printf("Provider: %s\n", result3b.Provider)

	fmt.Printf("\n💸 NEW monthly projection: $%.2f (saved $%.2f!)\n",
		result3.Cost*10000*30*0.3, // 70% queries are simple
		(result1.Cost-result3.Cost)*10000*30*0.7)
	time.Sleep(2 * time.Second)

	// Sprint 5: Provider Fallback.
	fmt.Println("\n\n📅 SPRINT 5: The Great Outage - Multi-Provider Fallback")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("3 AM: OpenAI is down! Support queue growing...")
	EnableProviderFallback()

	// Simulate OpenAI being down.
	GPT4Service.Behavior = BehaviorError
	GPT35Service.Behavior = BehaviorError

	query5 := SupportQuery{
		CustomerID: "CUST-002",
		Query:      "I need a refund for my damaged item",
		Channel:    "web",
	}

	result5, err := ProcessQuery(ctx, query5)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Fallback worked! Response in %v\n", result5.ResponseTime)
		fmt.Printf("Provider: %s (OpenAI was down!)\n", result5.Provider)
		fmt.Printf("Response: %.100s...\n", result5.Response)
	}

	// Reset providers.
	ResetAllProviders()
	fmt.Println("\n🛡️ Crisis averted! Support never went down!")
	time.Sleep(2 * time.Second)

	// Sprint 7: Speed Optimization.
	fmt.Println("\n\n📅 SPRINT 7: Speed Wars - Race for Fastest Response")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("CMO: 'TechCrunch says our competitor has 500ms responses!'")
	EnableSpeedOptimization()

	// Urgent query.
	query7 := SupportQuery{
		CustomerID: "CUST-003",
		Query:      "MY WEDDING IS TOMORROW AND MY DRESS HASN'T ARRIVED! WHERE IS ORDER #99999?!",
		Channel:    "mobile",
	}

	fmt.Println("\nProcessing urgent query...")
	result7, err := ProcessQuery(ctx, query7)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	fmt.Printf("⚡ URGENT query handled in: %v\n", result7.ResponseTime)
	fmt.Printf("Provider: %s (won the race!)\n", result7.Provider)
	fmt.Printf("Sentiment: %s, Urgency: %s\n", result7.Sentiment, result7.Urgency)

	// Compare with normal query.
	normalQuery := SupportQuery{
		CustomerID: "CUST-004",
		Query:      "How do I update my shipping address?",
		Channel:    "web",
	}

	resultNormal, err := ProcessQuery(ctx, normalQuery)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	fmt.Printf("\n📊 Normal query: %v (Provider: %s)\n", resultNormal.ResponseTime, resultNormal.Provider)
	fmt.Printf("Speed improvement for urgent: %.1f%% faster!\n",
		(float64(resultNormal.ResponseTime-result7.ResponseTime)/float64(resultNormal.ResponseTime))*100)
	time.Sleep(2 * time.Second)

	// Sprint 11: Production Features.
	fmt.Println("\n\n📅 SPRINT 11: Production Ready - All Features Enabled")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	EnableProductionFeatures()

	// Test various scenarios.
	testQueries := []SupportQuery{
		{
			CustomerID: "CUST-100",
			Query:      "Where is order #55555?",
			Channel:    "web",
		},
		{
			CustomerID: "CUST-101",
			Query:      "This is the WORST service ever! I want a refund NOW!",
			Channel:    "mobile",
		},
		{
			CustomerID: "CUST-102",
			Query:      "How do I integrate your API with my system?",
			Channel:    "email",
		},
	}

	fmt.Println("\nProcessing various query types...")
	for i := range testQueries {
		result, err := ProcessQuery(ctx, testQueries[i])
		if err != nil {
			fmt.Printf("\n❌ Query %d failed: %v\n", i+1, err)
		} else {
			fmt.Printf("\n✅ Query %d: %s\n", i+1, result.QueryType)
			fmt.Printf("   Time: %v, Provider: %s, Cost: $%.4f\n",
				result.ResponseTime, result.Provider, result.Cost)
			fmt.Printf("   Sentiment: %s, Urgency: %s\n", result.Sentiment, result.Urgency)
		}
	}

	// Show final metrics.
	fmt.Println("\n" + MetricsCollector.GetSummary().PerformanceReport())

	// Success story.
	fmt.Println("\n🎉 SUCCESS STORY")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅ Started with simple GPT-4 calls costing $50k/month")
	fmt.Println("✅ Now handling 10k+ queries/day with:")
	fmt.Println("   • 93% cost reduction through smart routing")
	fmt.Println("   • 60% faster responses for urgent queries")
	fmt.Println("   • 99.9% uptime with multi-provider fallback")
	fmt.Println("   • Automatic priority handling for angry customers")
	fmt.Println("   • Full metrics and monitoring")
	fmt.Println("\n🚀 From MVP to production in 11 sprints!")
}

func runSprint(ctx context.Context, sprintNum int) {
	fmt.Printf("Running Sprint %d Demo\n", sprintNum)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Reset and configure for specific sprint.
	ResetPipeline()
	ResponseCache.Clear()
	MetricsCollector.Clear()

	switch sprintNum {
	case 1:
		fmt.Println("Sprint 1: MVP - Basic GPT-4 integration")
	// Pipeline already in MVP state.

	case 3:
		fmt.Println("Sprint 3: Cost Optimization - Smart routing")
		EnableCostOptimization()

	case 5:
		fmt.Println("Sprint 5: Provider Fallback - Multi-provider resilience")
		EnableProviderFallback()

	case 7:
		fmt.Println("Sprint 7: Speed Optimization - Race for fastest response")
		EnableSpeedOptimization()

	case 9:
		fmt.Println("Sprint 9: Priority Routing - Sentiment-based handling")
		EnablePriorityRouting()

	case 11:
		fmt.Println("Sprint 11: Production Ready - All features")
		EnableProductionFeatures()

	default:
		fmt.Printf("Invalid sprint number: %d (valid: 1,3,5,7,9,11)\n", sprintNum)
		os.Exit(1)
	}

	// Run test queries.
	fmt.Println("\nTesting pipeline...")
	testQueries := []SupportQuery{
		{
			CustomerID: "TEST-001",
			Query:      "Where is my order #12345?",
			Channel:    "web",
		},
		{
			CustomerID: "TEST-002",
			Query:      "I want a refund for my damaged item",
			Channel:    "mobile",
		},
		{
			CustomerID: "TEST-003",
			Query:      "URGENT! MY PACKAGE HASN'T ARRIVED!",
			Channel:    "email",
		},
	}

	for i := range testQueries {
		fmt.Printf("\nQuery %d: %.50s...\n", i+1, testQueries[i].Query)

		result, err := ProcessQuery(ctx, testQueries[i])
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
		} else {
			fmt.Printf("✅ Success!\n")
			fmt.Printf("   Provider: %s\n", result.Provider)
			fmt.Printf("   Time: %v\n", result.ResponseTime)
			fmt.Printf("   Cost: $%.4f\n", result.Cost)

			// Only show classification info for sprints that have it.
			if sprintNum >= 3 && result.QueryType != QueryTypeUnknown {
				fmt.Printf("   Type: %s\n", result.QueryType)
			}
			if sprintNum >= 7 && result.Sentiment != SentimentNeutral {
				fmt.Printf("   Sentiment: %s\n", result.Sentiment)
			}
			if sprintNum >= 7 && result.Urgency != UrgencyNormal {
				fmt.Printf("   Urgency: %s\n", result.Urgency)
			}

			// Show cache hit for Sprint 3+.
			if sprintNum >= 3 && result.CacheHit {
				fmt.Printf("   Cache: HIT! (saved $%.4f)\n", result.Cost)
			}

			fmt.Printf("   Response: %.100s...\n", result.Response)
		}
	}

	// Show metrics if available.
	if sprintNum >= 11 {
		fmt.Println("\n" + MetricsCollector.GetSummary().PerformanceReport())
	}
}
