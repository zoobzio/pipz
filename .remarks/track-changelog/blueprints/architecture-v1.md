# Changelog Tracking & Automated Release Architecture

**Author:** midgel  
**Version:** v1  
**Date:** 2025-08-22  
**Branch:** feat/track-changelog  

---

## Executive Summary

The pipz project already has solid infrastructure. GoReleaser handles releases, GitHub Actions manage CI/CD, and there's a consistent conventional commit pattern. What's missing is **automatic changelog generation from commits** and **intelligent version inference**. 

Good news: we can build on existing patterns without breaking anything. Bad news: someone still has to implement it properly.

---

## Current State Analysis

### What's Already Working
✅ **GoReleaser Configuration**: Comprehensive setup with changelog grouping by commit type  
✅ **GitHub Actions**: Robust CI/CD pipeline with release automation  
✅ **Conventional Commits**: Consistent `feat:`, `fix:`, `docs:`, etc. patterns  
✅ **Version Tags**: Following semver (v1.0.1, v1.0.0, v0.0.1)  
✅ **Test Coverage**: Solid testing infrastructure with 80%+ coverage  

### What's Missing
❌ **Automatic Version Inference**: Manual tag creation required  
❌ **Commit-based Changelog**: GoReleaser only runs on tag push  
❌ **Pre-release Validation**: No version bump validation  
❌ **Breaking Change Detection**: No automatic major version handling  

---

## Technical Architecture

### Core Design Principles

1. **Leverage Existing Infrastructure**: Build on GoReleaser, don't replace it
2. **Follow pipz Patterns**: Use the same modular, composable approach
3. **Zero Breaking Changes**: Existing workflows continue working
4. **Fail-Safe Defaults**: Conservative version inference over aggressive

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Changelog System                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────┐    ┌─────────────────┐                │
│  │  Git Analysis   │    │ Version         │                │
│  │  Package        │────│ Inference       │                │
│  │                 │    │ Package         │                │
│  └─────────────────┘    └─────────────────┘                │
│           │                       │                        │
│           └───────┬───────────────┘                        │
│                   │                                        │
│           ┌─────────────────┐                              │
│           │  Changelog      │                              │
│           │  Generation     │                              │
│           │  Package        │                              │
│           └─────────────────┘                              │
│                   │                                        │
│           ┌─────────────────┐                              │
│           │  CLI Tool       │                              │
│           │  (cmd/changelog)│                              │
│           └─────────────────┘                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Package Structure

### New Packages Required

```
/internal/changelog/          # Core changelog functionality
├── git.go                   # Git repository analysis
├── git_test.go
├── version.go               # Semantic version inference
├── version_test.go  
├── generate.go              # Changelog generation logic
├── generate_test.go
└── types.go                 # Shared types and interfaces

/cmd/changelog/              # CLI tool for manual operations
├── main.go                  # CLI entry point
├── commands.go              # Command implementations
└── commands_test.go

/.github/workflows/          # Enhanced CI/CD (modify existing)
├── version-check.yml        # New: Pre-release version validation
└── ci.yml                   # Modified: Add changelog validation
```

### Integration Points

**Existing Files to Modify:**
- `.github/workflows/ci.yml` - Add changelog validation step
- `.github/workflows/release.yml` - Add version inference step  
- `Makefile` - Add changelog-related targets
- `.golangci.yml` - Ensure new packages follow linting standards

**Files to NOT Touch:**
- `.goreleaser.yml` - Already configured correctly
- Core library files (`*.go` in root) - No business logic changes
- Example packages - Remain unchanged

---

## Detailed Component Design

### 1. Git Analysis Package (`/internal/changelog/git.go`)

**Responsibility**: Extract commit information and classify changes

```go
// Core types
type CommitInfo struct {
    SHA         string
    Message     string
    Type        CommitType
    Scope       string
    Breaking    bool
    Body        string
    Timestamp   time.Time
}

type CommitType int
const (
    Feature CommitType = iota
    Fix
    Documentation
    Chore
    // ... etc
)

// Primary interface
type GitAnalyzer interface {
    GetCommitsSince(ctx context.Context, since string) ([]CommitInfo, error)
    GetLatestTag(ctx context.Context) (string, error)
    IsCleanWorkingDirectory(ctx context.Context) (bool, error)
}
```

**Key Functions:**
- Parse conventional commit messages with regex
- Extract commit metadata (author, timestamp, etc.)
- Identify breaking changes (`!` suffix or `BREAKING CHANGE:` footer)
- Handle edge cases (merge commits, invalid formats)

### 2. Version Inference Package (`/internal/changelog/version.go`)

**Responsibility**: Determine next version based on commit analysis

```go
type VersionInference struct {
    Current string
    Next    string
    Reason  []string
    Changes []CommitInfo
}

type VersionInferrer interface {
    InferNextVersion(ctx context.Context, commits []CommitInfo, currentVersion string) (*VersionInference, error)
}
```

**Version Rules** (Following Semantic Versioning):
- **MAJOR**: Any commit with `BREAKING CHANGE` or `!` suffix
- **MINOR**: Any `feat:` commit without breaking changes
- **PATCH**: Any `fix:` commit, or other types without features/breaking changes
- **SPECIAL**: Pre-release suffixes (-alpha, -beta, -rc) handled appropriately

### 3. Changelog Generation Package (`/internal/changelog/generate.go`)

**Responsibility**: Create formatted changelog content

```go
type ChangelogGenerator interface {
    GenerateChangelog(ctx context.Context, commits []CommitInfo, version string) (string, error)
    GenerateReleaseDiff(ctx context.Context, from, to string) (string, error)
}

type ChangelogEntry struct {
    Version   string
    Date      time.Time
    Sections  map[CommitType][]CommitInfo
    Breaking  []CommitInfo
}
```

**Output Formats:**
- **Markdown**: For CHANGELOG.md file
- **GitHub Release**: For release notes
- **JSON**: For programmatic consumption

### 4. CLI Tool (`/cmd/changelog/`)

**Commands:**
```bash
# Generate changelog for unreleased changes
go run ./cmd/changelog generate

# Preview next version without creating tag
go run ./cmd/changelog version

# Validate commit messages in range
go run ./cmd/changelog validate --since=v1.0.0

# Generate changelog between two tags
go run ./cmd/changelog diff v1.0.0..v1.0.1
```

---

## GitHub Actions Integration

### New Workflow: Version Check (`/.github/workflows/version-check.yml`)

```yaml
name: Version Check

on:
  pull_request:
    branches: [ main ]

jobs:
  version-preview:
    name: Preview Version Change
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Preview version change
        run: |
          go run ./cmd/changelog version --format=github >> $GITHUB_STEP_SUMMARY
          
      - name: Validate commit messages
        run: |
          go run ./cmd/changelog validate --since=origin/main
```

### Modified CI Workflow

Add to existing `.github/workflows/ci.yml`:

```yaml
  changelog-validation:
    name: Changelog Validation
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0
    
    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.23'
    
    - name: Build changelog tool
      run: go build -o changelog ./cmd/changelog
    
    - name: Validate commits since last tag
      run: ./changelog validate --since=$(git describe --tags --abbrev=0)
```

### Enhanced Release Workflow

Add to existing `.github/workflows/release.yml` (before GoReleaser):

```yaml
      - name: Generate and validate changelog
        run: |
          go run ./cmd/changelog version
          go run ./cmd/changelog generate > CHANGELOG_RELEASE.md
          echo "Generated changelog for release"
```

---

## Testing Strategy

### Unit Testing Requirements

**Coverage Targets:**
- `/internal/changelog/`: 95%+ (critical business logic)
- `/cmd/changelog/`: 80%+ (CLI commands)

**Test Categories:**

1. **Git Analysis Tests**
   - Conventional commit parsing (all formats)
   - Edge cases (malformed commits, merges)
   - Repository state detection
   - Mock git operations for deterministic tests

2. **Version Inference Tests**
   - All semantic versioning scenarios
   - Breaking change detection
   - Pre-release version handling
   - Version constraint edge cases

3. **Changelog Generation Tests**
   - Markdown formatting
   - Grouping by commit type
   - Date handling and sorting
   - Large commit volume handling

4. **CLI Integration Tests**
   - Command parsing and validation
   - Output format verification
   - Error handling and user feedback
   - Git repository integration

### Integration Testing

**Test Repository Setup:**
- Create test git repositories with known commit history
- Verify end-to-end workflow with GoReleaser
- Test GitHub Actions workflow modifications

---

## Error Handling & Edge Cases

### Expected Failure Scenarios

1. **Invalid Commit Messages**: Non-conventional format
   - **Action**: Warn but continue, categorize as "Other Changes"
   - **Rationale**: Don't break builds for formatting issues

2. **Missing Git Tags**: No previous version to compare against
   - **Action**: Start from v0.1.0 or user-specified base
   - **Rationale**: Handle new repositories gracefully

3. **Dirty Working Directory**: Uncommitted changes
   - **Action**: Fail with clear error message
   - **Rationale**: Ensure clean state for version inference

4. **Network Issues**: GitHub API calls fail
   - **Action**: Graceful degradation to local-only operation
   - **Rationale**: Core functionality shouldn't depend on external services

### Data Validation

- **Commit SHA validation**: Ensure valid git object references
- **Version format validation**: Strict semver compliance
- **Date parsing**: Handle various timestamp formats
- **Commit message encoding**: UTF-8 support for international contributors

---

## Performance Considerations

### Git Operations
- **Batch git commands**: Minimize subprocess calls
- **Shallow clones**: Only fetch necessary history
- **Caching**: Cache parsed commit data for repeated operations

### Memory Usage
- **Stream processing**: Handle large commit histories without loading all into memory
- **Lazy evaluation**: Parse commits only when needed
- **Resource limits**: Set reasonable limits on commit history depth

---

## Security Considerations

### Git Repository Security
- **Command injection**: Sanitize all git parameters
- **Path traversal**: Validate repository paths
- **Resource exhaustion**: Limit git operation timeouts

### Credential Management
- **No credential storage**: Use existing git authentication
- **Read-only operations**: No git modifications in library code
- **GitHub token**: Only for enhanced release notes (optional)

---

## Migration Strategy

### Phase 1: Core Implementation
1. Implement `/internal/changelog/` packages
2. Add comprehensive test suite
3. Create CLI tool with basic commands

### Phase 2: CI Integration  
1. Add version check workflow
2. Modify existing CI for validation
3. Test with feature branches

### Phase 3: Release Integration
1. Enhance release workflow
2. Validate with test releases
3. Document new processes

### Backward Compatibility
- **Existing releases**: No changes to tagged versions
- **Manual workflow**: Current manual tagging still works
- **GoReleaser**: All existing configuration preserved

---

## Success Metrics

### Functionality Metrics
- **Accurate version inference**: 100% correct for conventional commits
- **Changelog completeness**: All commits properly categorized
- **Performance**: Generate changelog in <5 seconds for 1000 commits

### Quality Metrics
- **Test coverage**: >90% overall, >95% for core logic
- **Documentation**: Every public function documented
- **Linting**: Zero issues with existing .golangci.yml

### Operational Metrics
- **CI time impact**: <30 seconds additional CI time
- **Release reliability**: Zero failed releases due to changelog issues
- **Developer experience**: Clear error messages, helpful CLI output

---

## Conclusion

This architecture leverages the existing solid foundation in pipz while adding intelligent changelog generation. The modular design follows pipz patterns, integrates cleanly with existing CI/CD, and provides room for future enhancements.

Key strengths:
- **Conservative approach**: Builds on proven infrastructure
- **Type-safe implementation**: Follows Go best practices
- **Comprehensive testing**: Ensures reliability
- **Clear separation**: New code isolated from core library

The implementation is straightforward but not trivial. Someone will need to write proper git parsing, implement semantic versioning logic, and ensure the GitHub Actions integration works correctly. But the architecture is sound, and the foundations are solid.

**Bottom line**: This will work, it won't break existing functionality, and it'll make releases significantly more reliable. Which is exactly what you'd expect from proper engineering.