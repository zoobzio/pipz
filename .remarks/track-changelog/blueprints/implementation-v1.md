# Implementation Blueprint: Changelog Tracking System

**Author:** midgel  
**Version:** v1  
**Date:** 2025-08-22  
**Branch:** feat/track-changelog  

---

## Implementation Priorities

Look, we need to be strategic about this. The architecture is solid, but kevin's going to need very specific instructions. Here's the implementation order that makes sense:

1. **Core Git Analysis** - Foundation for everything else
2. **Version Inference** - Critical logic, needs bulletproof testing  
3. **Changelog Generation** - Output formatting and templating
4. **CLI Tool** - User interface and command handling
5. **GitHub Actions Integration** - Workflow modifications
6. **Testing & Validation** - Comprehensive test coverage

---

## Package Implementation Details

### 1. Core Types (`/internal/changelog/types.go`)

Start here. These types drive everything else.

```go
package changelog

import (
    "context"
    "regexp"
    "time"
)

// CommitType represents the type of change in a conventional commit
type CommitType int

const (
    CommitTypeFeature CommitType = iota
    CommitTypeFix
    CommitTypeDocumentation
    CommitTypeStyle
    CommitTypeRefactor
    CommitTypePerformance
    CommitTypeTest
    CommitTypeChore
    CommitTypeBuild
    CommitTypeCI
    CommitTypeRevert
    CommitTypeOther
)

// String returns the string representation of the commit type
func (ct CommitType) String() string {
    switch ct {
    case CommitTypeFeature:
        return "feat"
    case CommitTypeFix:
        return "fix"
    case CommitTypeDocumentation:
        return "docs"
    case CommitTypeStyle:
        return "style"
    case CommitTypeRefactor:
        return "refactor"
    case CommitTypePerformance:
        return "perf"
    case CommitTypeTest:
        return "test"
    case CommitTypeChore:
        return "chore"
    case CommitTypeBuild:
        return "build"
    case CommitTypeCI:
        return "ci"
    case CommitTypeRevert:
        return "revert"
    default:
        return "other"
    }
}

// CommitInfo contains parsed information about a git commit
type CommitInfo struct {
    SHA          string    `json:"sha"`
    Message      string    `json:"message"`
    Subject      string    `json:"subject"`
    Body         string    `json:"body"`
    Type         CommitType `json:"type"`
    Scope        string    `json:"scope"`
    Breaking     bool      `json:"breaking"`
    Author       string    `json:"author"`
    AuthorEmail  string    `json:"author_email"`
    Timestamp    time.Time `json:"timestamp"`
    Refs         []string  `json:"refs"` // Associated issues/PRs
}

// VersionInfo represents version calculation results
type VersionInfo struct {
    Current    string        `json:"current"`
    Next       string        `json:"next"`
    BumpType   string        `json:"bump_type"` // major, minor, patch
    Reason     []string      `json:"reason"`
    Commits    []CommitInfo  `json:"commits"`
    HasChanges bool          `json:"has_changes"`
}

// ChangelogSection represents a grouped section of changes
type ChangelogSection struct {
    Title   string       `json:"title"`
    Type    CommitType   `json:"type"`
    Commits []CommitInfo `json:"commits"`
    Order   int          `json:"order"`
}

// ChangelogData contains all information needed to generate a changelog
type ChangelogData struct {
    Version      string             `json:"version"`
    Date         time.Time          `json:"date"`
    Sections     []ChangelogSection `json:"sections"`
    Breaking     []CommitInfo       `json:"breaking"`
    TotalCommits int                `json:"total_commits"`
    Repository   string             `json:"repository"`
}

// Interfaces define the contract for each component
type GitAnalyzer interface {
    GetCommitsSince(ctx context.Context, since string) ([]CommitInfo, error)
    GetLatestTag(ctx context.Context) (string, error)
    IsCleanWorkingDirectory(ctx context.Context) (bool, error)
    GetRepositoryURL(ctx context.Context) (string, error)
}

type VersionInferrer interface {
    InferNextVersion(ctx context.Context, commits []CommitInfo, currentVersion string) (*VersionInfo, error)
    ValidateVersion(version string) error
}

type ChangelogGenerator interface {
    GenerateChangelog(ctx context.Context, data *ChangelogData) (string, error)
    GenerateReleaseNotes(ctx context.Context, data *ChangelogData) (string, error)
}
```

**Why these types?**
- **Comprehensive commit info**: Everything needed for version inference and changelog generation
- **Type safety**: No magic strings, everything strongly typed
- **JSON serializable**: Can be used for debugging, caching, or API responses
- **Interface-driven**: Easy to mock for testing, swap implementations

### 2. Git Analysis Implementation (`/internal/changelog/git.go`)

This is where the heavy lifting happens. Git parsing needs to be bulletproof.

```go
package changelog

import (
    "bufio"
    "context"
    "fmt"
    "os/exec"
    "regexp"
    "strconv"
    "strings"
    "time"
)

var (
    // Conventional commit regex - matches the standard format
    conventionalCommitRegex = regexp.MustCompile(`^(?P<type>\w+)(?:\((?P<scope>[^)]+)\))?(?P<breaking>!)?: (?P<subject>.+)$`)
    
    // Breaking change patterns in commit body
    breakingChangeRegex = regexp.MustCompile(`(?i)BREAKING CHANGE:\s*(.+)`)
    
    // Issue/PR reference patterns
    issueRefRegex = regexp.MustCompile(`(?i)(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?):?\s*#(\d+)`)
)

// gitAnalyzer implements GitAnalyzer using git CLI
type gitAnalyzer struct {
    repoPath string
}

// NewGitAnalyzer creates a new git analyzer for the given repository path
func NewGitAnalyzer(repoPath string) GitAnalyzer {
    return &gitAnalyzer{repoPath: repoPath}
}

// GetCommitsSince retrieves all commits since the given reference
func (g *gitAnalyzer) GetCommitsSince(ctx context.Context, since string) ([]CommitInfo, error) {
    // Git command to get commit information in a parseable format
    args := []string{
        "log",
        "--pretty=format:%H%x00%an%x00%ae%x00%at%x00%s%x00%b%x00%D",
        "--no-merges", // Skip merge commits for cleaner changelog
    }
    
    if since != "" {
        args = append(args, fmt.Sprintf("%s..HEAD", since))
    }
    
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = g.repoPath
    
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("failed to get git commits: %w", err)
    }
    
    return g.parseCommits(string(output))
}

// GetLatestTag returns the most recent git tag
func (g *gitAnalyzer) GetLatestTag(ctx context.Context) (string, error) {
    cmd := exec.CommandContext(ctx, "git", "describe", "--tags", "--abbrev=0")
    cmd.Dir = g.repoPath
    
    output, err := cmd.Output()
    if err != nil {
        // No tags exist yet
        return "", nil
    }
    
    return strings.TrimSpace(string(output)), nil
}

// IsCleanWorkingDirectory checks if there are uncommitted changes
func (g *gitAnalyzer) IsCleanWorkingDirectory(ctx context.Context) (bool, error) {
    cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
    cmd.Dir = g.repoPath
    
    output, err := cmd.Output()
    if err != nil {
        return false, fmt.Errorf("failed to check git status: %w", err)
    }
    
    return len(strings.TrimSpace(string(output))) == 0, nil
}

// GetRepositoryURL returns the remote origin URL
func (g *gitAnalyzer) GetRepositoryURL(ctx context.Context) (string, error) {
    cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
    cmd.Dir = g.repoPath
    
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to get remote URL: %w", err)
    }
    
    return strings.TrimSpace(string(output)), nil
}

// parseCommits parses the git log output into CommitInfo structs
func (g *gitAnalyzer) parseCommits(output string) ([]CommitInfo, error) {
    var commits []CommitInfo
    
    // Split by commit boundaries (commits are separated by newlines)
    lines := strings.Split(output, "\n")
    
    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }
        
        commit, err := g.parseCommit(line)
        if err != nil {
            // Log the error but continue processing other commits
            continue
        }
        
        commits = append(commits, commit)
    }
    
    return commits, nil
}

// parseCommit parses a single commit line into CommitInfo
func (g *gitAnalyzer) parseCommit(line string) (CommitInfo, error) {
    // Split the git log format: SHA, author, email, timestamp, subject, body, refs
    parts := strings.Split(line, "\x00")
    if len(parts) < 6 {
        return CommitInfo{}, fmt.Errorf("invalid commit format")
    }
    
    sha := parts[0]
    author := parts[1]
    email := parts[2]
    timestampStr := parts[3]
    subject := parts[4]
    body := parts[5]
    refs := []string{}
    if len(parts) > 6 && parts[6] != "" {
        refs = strings.Split(parts[6], ", ")
    }
    
    timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
    if err != nil {
        return CommitInfo{}, fmt.Errorf("invalid timestamp: %w", err)
    }
    
    commit := CommitInfo{
        SHA:         sha,
        Message:     fmt.Sprintf("%s\n%s", subject, body),
        Subject:     subject,
        Body:        body,
        Author:      author,
        AuthorEmail: email,
        Timestamp:   time.Unix(timestamp, 0),
        Refs:        extractIssueRefs(body),
    }
    
    // Parse conventional commit format
    g.parseConventionalCommit(&commit)
    
    return commit, nil
}

// parseConventionalCommit extracts type, scope, and breaking change info
func (g *gitAnalyzer) parseConventionalCommit(commit *CommitInfo) {
    matches := conventionalCommitRegex.FindStringSubmatch(commit.Subject)
    if len(matches) == 0 {
        commit.Type = CommitTypeOther
        return
    }
    
    // Extract named groups
    typeStr := matches[1]
    scope := matches[3]
    breaking := matches[4] == "!"
    
    commit.Type = parseCommitType(typeStr)
    commit.Scope = scope
    commit.Breaking = breaking
    
    // Check for breaking change in body
    if !breaking && breakingChangeRegex.MatchString(commit.Body) {
        commit.Breaking = true
    }
}

// parseCommitType converts string to CommitType
func parseCommitType(typeStr string) CommitType {
    switch strings.ToLower(typeStr) {
    case "feat", "feature":
        return CommitTypeFeature
    case "fix":
        return CommitTypeFix
    case "docs", "doc":
        return CommitTypeDocumentation
    case "style":
        return CommitTypeStyle
    case "refactor":
        return CommitTypeRefactor
    case "perf", "performance":
        return CommitTypePerformance
    case "test", "tests":
        return CommitTypeTest
    case "chore":
        return CommitTypeChore
    case "build":
        return CommitTypeBuild
    case "ci":
        return CommitTypeCI
    case "revert":
        return CommitTypeRevert
    default:
        return CommitTypeOther
    }
}

// extractIssueRefs finds issue/PR references in commit message
func extractIssueRefs(message string) []string {
    matches := issueRefRegex.FindAllStringSubmatch(message, -1)
    var refs []string
    
    for _, match := range matches {
        if len(match) > 1 {
            refs = append(refs, "#"+match[1])
        }
    }
    
    return refs
}
```

**Implementation Notes:**
- **Git command safety**: All commands use context for cancellation
- **Error handling**: Continue processing on individual commit parse errors
- **Regex patterns**: Based on conventional commits specification
- **Performance**: Single git command gets all needed data
- **No external dependencies**: Uses only standard library

### 3. Version Inference (`/internal/changelog/version.go`)

Semantic version calculation. This logic needs to be absolutely correct.

```go
package changelog

import (
    "context"
    "fmt"
    "regexp"
    "strconv"
    "strings"
)

var (
    semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9\-\.]+))?(?:\+([a-zA-Z0-9\-\.]+))?$`)
)

// versionInferrer implements VersionInferrer
type versionInferrer struct{}

// NewVersionInferrer creates a new version inferrer
func NewVersionInferrer() VersionInferrer {
    return &versionInferrer{}
}

// InferNextVersion calculates the next version based on commits
func (v *versionInferrer) InferNextVersion(ctx context.Context, commits []CommitInfo, currentVersion string) (*VersionInfo, error) {
    if len(commits) == 0 {
        return &VersionInfo{
            Current:    currentVersion,
            Next:       currentVersion,
            BumpType:   "none",
            Reason:     []string{"No new commits"},
            Commits:    commits,
            HasChanges: false,
        }, nil
    }
    
    // Parse current version
    major, minor, patch, prerelease, err := v.parseVersion(currentVersion)
    if err != nil {
        return nil, fmt.Errorf("invalid current version: %w", err)
    }
    
    // Analyze commits to determine version bump
    bumpType, reasons := v.analyzeBumpType(commits)
    
    // Calculate next version
    nextMajor, nextMinor, nextPatch := major, minor, patch
    
    switch bumpType {
    case "major":
        nextMajor++
        nextMinor = 0
        nextPatch = 0
    case "minor":
        nextMinor++
        nextPatch = 0
    case "patch":
        nextPatch++
    }
    
    // Format next version (strip prerelease for now)
    nextVersion := fmt.Sprintf("v%d.%d.%d", nextMajor, nextMinor, nextPatch)
    
    return &VersionInfo{
        Current:    currentVersion,
        Next:       nextVersion,
        BumpType:   bumpType,
        Reason:     reasons,
        Commits:    commits,
        HasChanges: true,
    }, nil
}

// ValidateVersion checks if a version string is valid semver
func (v *versionInferrer) ValidateVersion(version string) error {
    if !semverRegex.MatchString(version) {
        return fmt.Errorf("invalid semantic version format: %s", version)
    }
    return nil
}

// parseVersion parses a semantic version string
func (v *versionInferrer) parseVersion(version string) (major, minor, patch int, prerelease string, err error) {
    if version == "" {
        // Default starting version
        return 0, 1, 0, "", nil
    }
    
    matches := semverRegex.FindStringSubmatch(version)
    if len(matches) == 0 {
        return 0, 0, 0, "", fmt.Errorf("invalid version format: %s", version)
    }
    
    major, err = strconv.Atoi(matches[1])
    if err != nil {
        return 0, 0, 0, "", err
    }
    
    minor, err = strconv.Atoi(matches[2])
    if err != nil {
        return 0, 0, 0, "", err
    }
    
    patch, err = strconv.Atoi(matches[3])
    if err != nil {
        return 0, 0, 0, "", err
    }
    
    if len(matches) > 4 {
        prerelease = matches[4]
    }
    
    return major, minor, patch, prerelease, nil
}

// analyzeBumpType determines what type of version bump is needed
func (v *versionInferrer) analyzeBumpType(commits []CommitInfo) (string, []string) {
    var reasons []string
    hasMajor := false
    hasMinor := false
    hasPatch := false
    
    for _, commit := range commits {
        if commit.Breaking {
            hasMajor = true
            reasons = append(reasons, fmt.Sprintf("Breaking change: %s", commit.Subject))
        } else if commit.Type == CommitTypeFeature {
            hasMinor = true
            reasons = append(reasons, fmt.Sprintf("New feature: %s", commit.Subject))
        } else if commit.Type == CommitTypeFix {
            hasPatch = true
            reasons = append(reasons, fmt.Sprintf("Bug fix: %s", commit.Subject))
        }
    }
    
    // Determine bump type (major takes precedence)
    if hasMajor {
        return "major", reasons
    } else if hasMinor {
        return "minor", reasons
    } else if hasPatch {
        return "patch", reasons
    }
    
    // If only docs/chore/etc changes, still bump patch
    return "patch", []string{"Other changes"}
}
```

**Critical Implementation Details:**
- **Semantic versioning**: Strict adherence to semver.org specification
- **Breaking change detection**: Both `!` suffix and `BREAKING CHANGE:` patterns
- **Default behavior**: Conservative patch bumps for non-feature changes
- **Version parsing**: Handles existing tags with or without `v` prefix

---

## CLI Tool Implementation (`/cmd/changelog/`)

Kevin needs a simple interface. One main file, clear commands, good error messages.

```go
// cmd/changelog/main.go
package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "time"
    
    "github.com/zoobzio/pipz/internal/changelog"
)

func main() {
    if len(os.Args) < 2 {
        printUsage()
        os.Exit(1)
    }
    
    command := os.Args[1]
    
    switch command {
    case "version":
        handleVersionCommand()
    case "generate":
        handleGenerateCommand()
    case "validate":
        handleValidateCommand()
    case "diff":
        handleDiffCommand()
    default:
        fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
        printUsage()
        os.Exit(1)
    }
}

func printUsage() {
    fmt.Printf(`changelog - Generate changelogs from conventional commits

Usage:
    changelog <command> [options]

Commands:
    version     Preview next version based on commits
    generate    Generate changelog for unreleased changes
    validate    Validate commit messages
    diff        Generate changelog between two references

Global Options:
    -repo PATH  Repository path (default: current directory)
    -format     Output format: text, json, github (default: text)

Examples:
    changelog version
    changelog generate -output CHANGELOG.md
    changelog validate -since v1.0.0
    changelog diff v1.0.0..v1.1.0
`)
}

func handleVersionCommand() {
    var (
        repoPath = flag.String("repo", ".", "Repository path")
        format   = flag.String("format", "text", "Output format (text, json, github)")
        current  = flag.String("current", "", "Current version (auto-detected if empty)")
    )
    flag.CommandLine.Parse(os.Args[2:])
    
    ctx := context.Background()
    
    analyzer := changelog.NewGitAnalyzer(*repoPath)
    inferrer := changelog.NewVersionInferrer()
    
    // Get current version
    currentVersion := *current
    if currentVersion == "" {
        tag, err := analyzer.GetLatestTag(ctx)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error getting latest tag: %v\n", err)
            os.Exit(1)
        }
        currentVersion = tag
    }
    
    // Get commits since last tag
    commits, err := analyzer.GetCommitsSince(ctx, currentVersion)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error getting commits: %v\n", err)
        os.Exit(1)
    }
    
    // Infer next version
    versionInfo, err := inferrer.InferNextVersion(ctx, commits, currentVersion)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error inferring version: %v\n", err)
        os.Exit(1)
    }
    
    // Output based on format
    switch *format {
    case "json":
        json.NewEncoder(os.Stdout).Encode(versionInfo)
    case "github":
        printGitHubSummary(versionInfo)
    default:
        printVersionSummary(versionInfo)
    }
}

func printVersionSummary(info *changelog.VersionInfo) {
    fmt.Printf("Current Version: %s\n", info.Current)
    fmt.Printf("Next Version:    %s\n", info.Next)
    fmt.Printf("Bump Type:       %s\n", info.BumpType)
    fmt.Printf("Changes:         %d commits\n", len(info.Commits))
    
    if len(info.Reason) > 0 {
        fmt.Println("\nReasons:")
        for _, reason := range info.Reason {
            fmt.Printf("  • %s\n", reason)
        }
    }
}

func printGitHubSummary(info *changelog.VersionInfo) {
    fmt.Printf("## Version Preview\n\n")
    fmt.Printf("**Current:** %s → **Next:** %s (%s bump)\n\n", 
        info.Current, info.Next, info.BumpType)
    
    if len(info.Commits) > 0 {
        fmt.Printf("### Changes (%d commits)\n\n", len(info.Commits))
        for _, commit := range info.Commits {
            breaking := ""
            if commit.Breaking {
                breaking = " ⚠️"
            }
            fmt.Printf("- **%s**: %s%s\n", commit.Type.String(), commit.Subject, breaking)
        }
    }
}

// Additional command handlers would go here...
// handleGenerateCommand(), handleValidateCommand(), handleDiffCommand()
```

**CLI Design Principles:**
- **Single binary**: Everything in one executable
- **Familiar patterns**: Follows git/go CLI conventions
- **Multiple formats**: Text for humans, JSON for automation, GitHub for actions
- **Clear errors**: Helpful messages when things go wrong
- **Reasonable defaults**: Works without configuration

---

## GitHub Actions Integration

### Modify `.github/workflows/ci.yml`

Add this job to the existing workflow:

```yaml
  changelog-validation:
    name: Validate Changelog
    runs-on: ubuntu-latest
    if: github.event_name == 'pull_request'
    steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0  # Need full history for version comparison
    
    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.23'
    
    - name: Build changelog tool
      run: |
        cd cmd/changelog
        go build -o ../../changelog .
    
    - name: Preview version change
      run: |
        echo "## 📋 Changelog Preview" >> $GITHUB_STEP_SUMMARY
        echo "" >> $GITHUB_STEP_SUMMARY
        ./changelog version -format github >> $GITHUB_STEP_SUMMARY
    
    - name: Validate commit messages
      run: |
        # Get base branch for PR
        BASE_REF=$(git merge-base HEAD origin/main)
        ./changelog validate -since $BASE_REF || {
            echo "❌ Some commit messages don't follow conventional commit format"
            echo "See: https://www.conventionalcommits.org/"
            exit 1
        }
```

### Modify `.github/workflows/release.yml`

Add before the GoReleaser step:

```yaml
      - name: Generate release changelog
        run: |
          cd cmd/changelog
          go build -o ../../changelog .
          cd ../..
          
          # Generate changelog for this release
          ./changelog generate -output CHANGELOG_RELEASE.md
          
          echo "## 📋 Generated Changelog" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          cat CHANGELOG_RELEASE.md >> $GITHUB_STEP_SUMMARY
      
      - name: Validate version consistency
        run: |
          # Ensure the tag matches what our tool would generate
          EXPECTED_VERSION=$(./changelog version -format json | jq -r '.next')
          ACTUAL_VERSION=${GITHUB_REF#refs/tags/}
          
          if [ "$EXPECTED_VERSION" != "$ACTUAL_VERSION" ]; then
            echo "❌ Version mismatch!"
            echo "Expected: $EXPECTED_VERSION"
            echo "Actual: $ACTUAL_VERSION"
            exit 1
          fi
          
          echo "✅ Version $ACTUAL_VERSION is correct"
```

---

## Testing Implementation Strategy

### Test Structure

```
/internal/changelog/
├── git_test.go              # Git operations testing
├── version_test.go          # Version inference testing
├── generate_test.go         # Changelog generation testing
├── testdata/
│   ├── commits.json         # Test commit data
│   ├── expected_changelog.md # Expected output
│   └── test_repo/           # Git repository for integration tests
└── test_helpers.go          # Common test utilities
```

### Critical Test Cases

**Git Analysis Tests:**
```go
func TestParseConventionalCommit(t *testing.T) {
    tests := []struct {
        name     string
        message  string
        expected CommitInfo
    }{
        {
            name:    "feature commit",
            message: "feat(auth): add OAuth2 support",
            expected: CommitInfo{
                Type:     CommitTypeFeature,
                Scope:    "auth",
                Subject:  "feat(auth): add OAuth2 support",
                Breaking: false,
            },
        },
        {
            name:    "breaking change with exclamation",
            message: "feat!: remove deprecated API",
            expected: CommitInfo{
                Type:     CommitTypeFeature,
                Breaking: true,
            },
        },
        {
            name:    "breaking change in body",
            message: "feat: new feature\n\nBREAKING CHANGE: removes old behavior",
            expected: CommitInfo{
                Type:     CommitTypeFeature,
                Breaking: true,
            },
        },
        // ... more test cases
    }
    
    analyzer := NewGitAnalyzer(".")
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            commit := CommitInfo{Subject: tt.message}
            analyzer.(*gitAnalyzer).parseConventionalCommit(&commit)
            
            assert.Equal(t, tt.expected.Type, commit.Type)
            assert.Equal(t, tt.expected.Breaking, commit.Breaking)
            assert.Equal(t, tt.expected.Scope, commit.Scope)
        })
    }
}
```

**Version Inference Tests:**
```go
func TestInferNextVersion(t *testing.T) {
    tests := []struct {
        name           string
        currentVersion string
        commits        []CommitInfo
        expectedNext   string
        expectedBump   string
    }{
        {
            name:           "patch bump for fix",
            currentVersion: "v1.0.0",
            commits: []CommitInfo{
                {Type: CommitTypeFix, Subject: "fix: bug"},
            },
            expectedNext: "v1.0.1",
            expectedBump: "patch",
        },
        {
            name:           "minor bump for feature",
            currentVersion: "v1.0.0",
            commits: []CommitInfo{
                {Type: CommitTypeFeature, Subject: "feat: new feature"},
            },
            expectedNext: "v1.1.0",
            expectedBump: "minor",
        },
        {
            name:           "major bump for breaking change",
            currentVersion: "v1.0.0",
            commits: []CommitInfo{
                {Type: CommitTypeFeature, Breaking: true, Subject: "feat!: breaking"},
            },
            expectedNext: "v2.0.0",
            expectedBump: "major",
        },
        // ... more test cases
    }
    
    inferrer := NewVersionInferrer()
    ctx := context.Background()
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := inferrer.InferNextVersion(ctx, tt.commits, tt.currentVersion)
            require.NoError(t, err)
            assert.Equal(t, tt.expectedNext, result.Next)
            assert.Equal(t, tt.expectedBump, result.BumpType)
        })
    }
}
```

---

## Error Handling Patterns

Follow the existing pipz error handling patterns:

```go
// Use pipz.Error[T] pattern for consistency
type ChangelogError struct {
    Operation string
    Err       error
    Context   map[string]interface{}
}

func (e *ChangelogError) Error() string {
    return fmt.Sprintf("changelog %s failed: %v", e.Operation, e.Err)
}

func (e *ChangelogError) Unwrap() error {
    return e.Err
}

// Wrap errors with context
func wrapError(op string, err error, context map[string]interface{}) error {
    return &ChangelogError{
        Operation: op,
        Err:       err,
        Context:   context,
    }
}
```

---

## Performance Requirements

**Git Operations:**
- Target: <2 seconds for 1000 commits
- Optimization: Single git command with custom format
- Caching: Parse commits once, reuse results

**Memory Usage:**
- Target: <50MB for 10,000 commits
- Strategy: Stream processing where possible
- Limits: Cap git log output at reasonable history depth

**CLI Responsiveness:**
- Target: <5 seconds for any command
- Implementation: Context timeouts on all operations
- User feedback: Progress indicators for long operations

---

## Deployment Strategy

### Phase 1: Core Implementation (Week 1)
1. Implement types and interfaces
2. Create git analysis package with tests
3. Implement version inference with comprehensive tests
4. Create basic CLI tool

### Phase 2: Integration (Week 2)
1. Add changelog generation
2. Integrate with GitHub Actions
3. Test with example releases
4. Documentation updates

### Phase 3: Validation (Week 3)
1. Comprehensive testing with real commit history
2. Performance optimization
3. Error handling refinement
4. Final documentation

**Success Criteria:**
- All tests passing with >95% coverage
- CLI tool handles all documented use cases
- GitHub Actions integration works correctly
- No breaking changes to existing functionality
- Performance targets met

---

## Conclusion

This implementation blueprint provides everything kevin needs to build the changelog system correctly. The patterns follow Go best practices, integrate cleanly with the existing pipz codebase, and build on the solid CI/CD foundation already in place.

Key implementation points:
- **Start with types**: Solid foundation prevents later refactoring
- **Test everything**: Each component needs comprehensive tests
- **Follow pipz patterns**: Consistency with existing codebase
- **CLI-first design**: Tool should work standalone before GitHub integration
- **Conservative version inference**: Better to under-bump than over-bump

The architecture is sound, the implementation is straightforward, and the integration points are well-defined. This will work reliably and won't break existing functionality.

Now someone just needs to write the code.