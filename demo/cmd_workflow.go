package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"pipz"
	"pipz/demo/testutil"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Multi-Stage Processing Workflow demonstration",
	Long:  `Demonstrates combining multiple contracts for complex workflows.`,
	Run:   runWorkflowDemo,
}

func init() {
	rootCmd.AddCommand(workflowCmd)
}

// Demo types for workflow
type WorkflowStage string

const (
	ValidationStage  WorkflowStage = "validation"
	EnrichmentStage  WorkflowStage = "enrichment"
	PersistenceStage WorkflowStage = "persistence"
)

type WorkflowUser struct {
	ID       string
	Email    string
	Name     string
	Age      int
	Country  string
	Verified bool
}

type WorkflowData struct {
	User      WorkflowUser
	Processed []string
	
	// Metadata fields as concrete types
	EnrichedAt       time.Time
	CountryDefaulted bool
	RiskScore        string
	Persisted        bool
	Database         string
	Shard            string
}

// Stage processors
func validateUser(w WorkflowData) ([]byte, error) {
	if w.User.Email == "" {
		return nil, fmt.Errorf("email required")
	}
	if !strings.Contains(w.User.Email, "@") {
		return nil, fmt.Errorf("invalid email format")
	}
	if w.User.Age < 18 {
		return nil, fmt.Errorf("user must be 18 or older")
	}
	w.Processed = append(w.Processed, "validated")
	return pipz.Encode(w)
}

func enrichUserData(w WorkflowData) ([]byte, error) {
	// Simulate data enrichment
	w.EnrichedAt = time.Now()
	
	// Mock IP geolocation
	if w.User.Country == "" {
		w.User.Country = "US" // Default country
		w.CountryDefaulted = true
	}
	
	// Add risk score based on email domain
	emailDomain := strings.Split(w.User.Email, "@")[1]
	if strings.Contains(emailDomain, "temp") || strings.Contains(emailDomain, "disposable") {
		w.RiskScore = "high"
	} else {
		w.RiskScore = "low"
	}
	
	w.Processed = append(w.Processed, "enriched")
	return pipz.Encode(w)
}

func persistUser(w WorkflowData) ([]byte, error) {
	// Simulate database save
	userID := fmt.Sprintf("USR-%d", time.Now().Unix())
	w.User.ID = userID
	w.User.Verified = true
	
	w.Persisted = true
	w.Database = "primary"
	w.Shard = userID[:3]
	
	w.Processed = append(w.Processed, "persisted")
	return pipz.Encode(w)
}

func runWorkflowDemo(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	
	// Get interactive flag from parent command
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	pp.Section("🔄 MULTI-STAGE PROCESSING WORKFLOW DEMO")
	
	pp.SubSection("📋 Use Case: User Registration Workflow")
	pp.Info("Scenario: A SaaS platform needs a multi-stage user registration process.")
	pp.Info("Requirements:")
	pp.Info("  • Stage 1: Validate user data (email, age)")
	pp.Info("  • Stage 2: Enrich with geolocation and risk scoring")
	pp.Info("  • Stage 3: Persist to database with sharding")
	pp.Info("  • Each stage should be independently testable")
	
	pp.SubSection("🔧 Workflow Architecture")
	pp.Code("go", `// Define stages as separate contracts
type WorkflowStage string

const (
    ValidationStage  WorkflowStage = "validation"
    EnrichmentStage  WorkflowStage = "enrichment"
    PersistenceStage WorkflowStage = "persistence"
)

// Create and compose the workflow
func setupWorkflow() *pipz.Chain[WorkflowData] {
    // Stage 1: Validation
    validationContract := pipz.GetContract[WorkflowData](ValidationStage)
    validationContract.Register(validateUser)
    
    // Stage 2: Enrichment
    enrichmentContract := pipz.GetContract[WorkflowData](EnrichmentStage)
    enrichmentContract.Register(enrichUserData)
    
    // Stage 3: Persistence
    persistenceContract := pipz.GetContract[WorkflowData](PersistenceStage)
    persistenceContract.Register(persistUser)
    
    // Compose into workflow
    chain := pipz.NewChain[WorkflowData]()
    chain.Add(
        validationContract.Link(),
        enrichmentContract.Link(),
        persistenceContract.Link(),
    )
    return chain
}`)
	
	// Set up the workflow stages
	pp.SubSection("Step 1: Register Individual Stages")
	
	// Validation stage
	validationContract := pipz.GetContract[WorkflowData](ValidationStage)
	validationContract.Register(validateUser)
	pp.Success("✓ Validation stage registered")
	
	// Enrichment stage
	enrichmentContract := pipz.GetContract[WorkflowData](EnrichmentStage)
	enrichmentContract.Register(enrichUserData)
	pp.Success("✓ Enrichment stage registered")
	
	// Persistence stage
	persistenceContract := pipz.GetContract[WorkflowData](PersistenceStage)
	persistenceContract.Register(persistUser)
	pp.Success("✓ Persistence stage registered")
	
	pp.SubSection("Step 2: Compose Into Workflow")
	
	// Create the workflow chain
	workflow := pipz.NewChain[WorkflowData]()
	workflow.Add(
		validationContract.Link(),
		enrichmentContract.Link(),
		persistenceContract.Link(),
	)
	
	pp.Success("✓ Workflow chain created with 3 stages")
	
	pp.SubSection("🔍 Live Workflow Execution")
	
	// Example 1: Valid user
	pp.Info("Example 1: Valid user registration")
	validUser := WorkflowData{
		User: WorkflowUser{
			Email: "john.doe@company.com",
			Name:  "John Doe",
			Age:   25,
		},
		Processed: []string{},
	}
	
	result, err := workflow.Process(validUser)
	if err != nil {
		pp.Error(fmt.Sprintf("Workflow failed: %v", err))
	} else {
		pp.Success("User registered successfully!")
		pp.Info(fmt.Sprintf("  User ID: %s", result.User.ID))
		pp.Info(fmt.Sprintf("  Verified: %v", result.User.Verified))
		pp.Info(fmt.Sprintf("  Country: %s", result.User.Country))
		pp.Info(fmt.Sprintf("  Risk Score: %s", result.RiskScore))
		pp.Info(fmt.Sprintf("  Database Shard: %s", result.Shard))
		pp.Info(fmt.Sprintf("  Stages Completed: %v", result.Processed))
	}
	
	pp.WaitForEnter("")
	
	pp.Info("")
	pp.Info("Example 2: Invalid user (underage)")
	underageUser := WorkflowData{
		User: WorkflowUser{
			Email: "kid@school.edu",
			Name:  "Young Kid",
			Age:   16, // Under 18
		},
		Processed: []string{},
	}
	
	_, err = workflow.Process(underageUser)
	if err != nil {
		pp.Error(fmt.Sprintf("Workflow failed at validation: %v", err))
		pp.Info("  ↳ Workflow stopped at first stage")
		pp.Info("  ↳ No data was enriched or persisted")
	}
	
	pp.Info("")
	pp.Info("Example 3: High-risk user")
	riskyUser := WorkflowData{
		User: WorkflowUser{
			Email: "user@temp-mail.com", // Disposable email
			Name:  "Suspicious User",
			Age:   19,
		},
		Processed: []string{},
	}
	
	result, err = workflow.Process(riskyUser)
	if err != nil {
		pp.Error(fmt.Sprintf("Workflow failed: %v", err))
	} else {
		pp.Warning("User registered with high risk score")
		pp.Info(fmt.Sprintf("  Risk Score: %s", result.RiskScore))
		pp.Info("  ↳ Disposable email domain detected")
	}
	
	pp.SubSection("🎯 Workflow Benefits")
	
	pp.Feature("🧩", "Modular Stages", "Each stage can be developed and tested independently")
	pp.Feature("🔄", "Reusable Contracts", "Stages can be reused in different workflows")
	pp.Feature("🛑", "Early Termination", "Workflow stops at first error, saving resources")
	pp.Feature("📊", "Stage Tracking", "Know exactly which stages were completed")
	pp.Feature("🔌", "Pluggable", "Easy to add, remove, or reorder stages")
	
	pp.SubSection("Stage Independence")
	pp.Info("Each stage is a separate contract:")
	pp.Info("- Can be tested in isolation")
	pp.Info("- Can be versioned independently")
	pp.Info("- Can be reused in other workflows")
	pp.Info("- Can be maintained by different teams")
	
	pp.SubSection("🔄 Alternative Workflow Patterns")
	
	pp.Info("You can also retrieve and modify stages dynamically:")
	pp.Code("go", `// Retrieve any stage by its key
enrichStage := pipz.GetContract[WorkflowData](EnrichmentStage)

// Add conditional stages
if user.Country == "EU" {
    gdprContract := pipz.GetContract[WorkflowData](GDPRStage)
    workflow.Add(gdprContract.Link())
}`)
	
	pp.Stats("Workflow Metrics", map[string]interface{}{
		"Total Stages": 3,
		"Avg Stage Time": "< 1ms",
		"Success Rate": "Depends on input",
		"Modularity": "100%",
	})
}