# Revised Requirements: Pragmatic Release Automation
**Version 2.0** | **Feature:** track-changelog | **Date:** 2025-08-22

## Executive Summary

**MISSION FAILURE ANALYSIS COMPLETE**

Our initial reconnaissance revealed a classic case of solution-first thinking leading to massive overengineering. The original mission parameters called for building a comprehensive changelog tracking system from scratch when the real problem is much simpler: **automatic version inference and tag creation**.

**REVISED MISSION OBJECTIVE:** Implement minimal release automation using existing tools, completing the final 5% gap in an otherwise excellent release pipeline.

**NEW STRATEGIC VALUE:** Achieve 100% automated releases in hours, not weeks, by leveraging battle-tested tools rather than building new ones.

---

## What Went Wrong: Post-Mortem Analysis

### The Original Plan vs Reality

**Original Plan (15-20 days of work):**
- Build custom git analysis package
- Implement semantic version inference engine
- Create changelog generation system
- Build CLI tool with multiple commands
- Design complex GitHub Actions integration
- Comprehensive testing framework
- **Total: 3,000+ lines of new code**

**Reality Check:**
- GoReleaser already generates perfect changelogs
- Version inference can be solved with existing tools
- GitHub Actions workflow_dispatch already exists
- **Total needed: ~50 lines of YAML changes**

### Why We Overengineered

1. **Requirements Scope Creep**: Started with "automatic version inference" and expanded to "comprehensive changelog system"
2. **Not-Invented-Here Syndrome**: Assumed we needed custom solutions instead of researching existing tools
3. **Architecture-First Thinking**: midgel designed a complex system before understanding the actual problem
4. **Missing Environmental Analysis**: Failed to properly assess what GoReleaser already provides

### The Real Problem

The ONLY missing piece is automatic version inference for tag creation. Everything else already works perfectly:

- ✅ **Changelog Generation**: GoReleaser handles this expertly
- ✅ **Release Notes**: Already formatted beautifully
- ✅ **GitHub Integration**: Release workflow already exists
- ✅ **Commit Parsing**: GoReleaser groups by conventional commits
- ❌ **Version Inference**: Manual tag creation required

---

## Current State Analysis (Corrected)

### What GoReleaser Already Provides (PERFECTLY)

From `.goreleaser.yml` analysis:

```yaml
changelog:
  use: github
  sort: asc
  abbrev: 7
  groups:
    - title: "✨ Features"
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
    - title: "🐛 Bug Fixes"  
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
    - title: "📚 Documentation"
      regexp: '^.*?docs?(\([[:word:]]+\))??!?:.+$'
    # ... comprehensive commit categorization
```

**GoReleaser ALREADY:**
- Parses conventional commits
- Groups changes by type with beautiful formatting
- Generates release notes with proper headers/footers
- Handles GitHub release creation
- Manages checksums and archives
- Filters merge commits and WIP commits
- Supports prerelease detection

### What's Actually Missing

**ONLY THIS:** Automatic version inference to create the git tag that triggers GoReleaser.

That's it. That's the entire gap.

---

## Revised Requirements: Minimal Viable Solution

### Outcome 1: Automatic Version Inference
**Success Criteria:** System determines next version (patch/minor/major) from commits since last tag
**Implementation:** Use existing tool like `svu` or `semantic-release`
**Effort:** 2-4 hours

### Outcome 2: One-Click Release Triggering  
**Success Criteria:** GitHub Actions workflow_dispatch allows authorized release triggering
**Implementation:** Add workflow_dispatch to existing release.yml
**Effort:** 30 minutes

### Outcome 3: Tag Creation Integration
**Success Criteria:** Release workflow creates git tag automatically before running GoReleaser
**Implementation:** Add version inference + git tag steps to workflow
**Effort:** 1-2 hours

**TOTAL EFFORT: 4-6 hours maximum**

---

## Recommended Implementation Strategy

### Option A: Using `svu` (Recommended)
**Tool:** https://github.com/caarlos0/svu
**Author:** Same person who created GoReleaser
**Integration:** Designed specifically for this use case

```yaml
# Add to .github/workflows/release.yml
- name: Get next version
  id: version
  run: |
    VERSION=$(svu next)
    echo "version=$VERSION" >> $GITHUB_OUTPUT
    
- name: Create release tag
  run: |
    git tag ${{ steps.version.outputs.version }}
    git push origin ${{ steps.version.outputs.version }}
```

### Option B: Using `semantic-release`
**Tool:** Industry standard with massive ecosystem
**Benefits:** More features, plugin system
**Complexity:** Requires Node.js dependency

### Option C: Custom Script (Simplest)
**Implementation:** 20-line bash script using git and semver logic
**Benefits:** Zero external dependencies
**Maintenance:** Minimal, but we own the logic

---

## Implementation Details

### Enhanced Release Workflow

Modify `.github/workflows/release.yml` to add workflow_dispatch and version inference:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'
  workflow_dispatch:
    inputs:
      version_override:
        description: 'Override version (leave empty for auto-inference)'
        required: false
        type: string

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0
    
    # NEW: Version inference for workflow_dispatch
    - name: Determine version
      id: version
      if: github.event_name == 'workflow_dispatch'
      run: |
        if [ -n "${{ github.event.inputs.version_override }}" ]; then
          VERSION="${{ github.event.inputs.version_override }}"
        else
          # Install svu
          go install github.com/caarlos0/svu@latest
          VERSION=$(svu next)
        fi
        echo "version=$VERSION" >> $GITHUB_OUTPUT
        echo "Next version: $VERSION"
    
    # NEW: Create tag for workflow_dispatch
    - name: Create release tag
      if: github.event_name == 'workflow_dispatch'
      run: |
        git config user.name "github-actions[bot]"
        git config user.email "github-actions[bot]@users.noreply.github.com"
        git tag ${{ steps.version.outputs.version }}
        git push origin ${{ steps.version.outputs.version }}
    
    # EXISTING: GoReleaser step (unchanged)
    - name: Run GoReleaser
      uses: goreleaser/goreleaser-action@v5
      with:
        distribution: goreleaser
        version: latest
        args: release --clean
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Preview Workflow (Optional)

Add `.github/workflows/version-preview.yml`:

```yaml
name: Version Preview

on:
  pull_request:
    branches: [ main ]

jobs:
  preview:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0
    
    - name: Preview next version
      run: |
        go install github.com/caarlos0/svu@latest
        CURRENT=$(git describe --tags --abbrev=0)
        NEXT=$(svu next)
        echo "## 📋 Version Preview" >> $GITHUB_STEP_SUMMARY
        echo "" >> $GITHUB_STEP_SUMMARY  
        echo "**Current:** $CURRENT → **Next:** $NEXT" >> $GITHUB_STEP_SUMMARY
        echo "" >> $GITHUB_STEP_SUMMARY
        echo "### Changes since $CURRENT" >> $GITHUB_STEP_SUMMARY
        git log --oneline $CURRENT..HEAD >> $GITHUB_STEP_SUMMARY
```

---

## Success Criteria (Realistic)

### Functional Requirements
1. **Version Inference Accuracy**: 100% correct for conventional commits
2. **Release Triggering**: Authorized users can trigger releases via GitHub UI
3. **Tag Creation**: System creates correct git tags automatically
4. **GoReleaser Integration**: Existing changelog generation unchanged

### Quality Requirements
1. **Zero Breaking Changes**: All existing functionality preserved
2. **Minimal Dependencies**: Prefer tools already in Go ecosystem
3. **Fast Implementation**: Complete solution in 4-6 hours
4. **Low Maintenance**: No complex custom code to maintain

### Operational Requirements
1. **One-Click Releases**: From GitHub Actions tab
2. **Version Preview**: PR comments show next version
3. **Error Handling**: Clear failures with actionable messages
4. **Documentation**: Updated README with new release process

---

## Risk Mitigation

### Risk 1: Tool Dependency
**Mitigation:** Use `svu` (same author as GoReleaser, proven track record)
**Contingency:** Custom 20-line bash script as fallback

### Risk 2: Version Conflicts
**Mitigation:** Check for existing tags before creation
**Contingency:** Manual override input for edge cases

### Risk 3: Workflow Permission Issues
**Mitigation:** Test with repository admin permissions
**Contingency:** Use Personal Access Token if needed

---

## Lessons Learned: Requirements Gathering Anti-Patterns

### What Went Wrong

1. **Solution-First Requirements**
   - Started with "we need a changelog system" 
   - Should have started with "we manually create tags"

2. **Insufficient Environmental Analysis**
   - Didn't fully assess existing GoReleaser capabilities
   - Assumed gaps existed where they didn't

3. **Scope Expansion Without Validation**
   - Grew from "version inference" to "comprehensive system"
   - No checkpoint to validate expanded scope

4. **Architecture Before Problem Understanding**
   - midgel designed complex solutions before confirming need
   - Should have researched existing tools first

### How to Avoid This in Future

1. **Problem-First Analysis**
   - Always start with "what exactly is broken?"
   - Document current state comprehensively before solutions

2. **Existing Tool Research**
   - Spend time researching ecosystem solutions
   - Prefer composition over creation

3. **Minimum Viable Outcome**
   - Define smallest possible success
   - Expand only after achieving MVP

4. **Regular Scope Validation**
   - Checkpoint every solution against original problem
   - Challenge complexity before implementation

### Requirements Definition Improvements

1. **Environmental Assessment**
   - ALWAYS audit existing tools and infrastructure first
   - Document what already works perfectly

2. **Solution Research Phase**
   - Dedicated time for researching existing solutions
   - Cost/benefit analysis of build vs buy vs integrate

3. **Complexity Budgets**
   - Set maximum complexity limits upfront
   - Require justification for any solution over X lines of code

4. **Implementation Time Reality Checks**
   - If estimate is weeks/months, challenge the approach
   - Simple problems should have simple solutions

---

## Technical Delegation

### For midgel (Chief Engineer)
**Assignment:** Validate the technical approach and implementation details

**Specific Requirements:**
- Review `svu` tool for security and reliability
- Design fallback approach if external tool fails
- Validate GitHub Actions workflow changes won't break existing releases
- Create testing strategy for the workflow_dispatch integration

**Success Criteria:** Technical validation that confirms approach is sound and risk-free

### For fidgel (Intelligence Officer)
**Assignment:** Research alternative approaches and validate this solution against industry best practices

**Specific Requirements:**
- Compare `svu` vs `semantic-release` vs custom script approaches
- Research how other Go projects handle automated releases
- Identify any gotchas or edge cases we haven't considered
- Validate our lessons learned against industry patterns

**Success Criteria:** Confirmation that our simplified approach aligns with industry best practices

### For kevin (Implementation Specialist)
**Assignment:** Implement the minimal workflow changes

**Specific Requirements:**
- Modify `.github/workflows/release.yml` with workflow_dispatch support
- Add version inference and tag creation steps
- Create the optional version preview workflow
- Test the complete workflow in a safe environment

**Success Criteria:** Working implementation that allows one-click releases via GitHub UI

---

## Conclusion

The original 15-20 day complex system has been replaced with a 4-6 hour integration of existing tools. This revision demonstrates the critical importance of:

1. **Understanding the problem deeply** before designing solutions
2. **Researching existing tools** before building new ones  
3. **Questioning complexity** when simple problems emerge
4. **Validating scope** against original business needs

GoReleaser already provides an excellent changelog and release system. We just need to complete the automation chain with version inference and tag creation. 

**Mission Parameters Revised:** Achieve the same business outcome with 99% less code and 95% less time.

This is how engineering should work.

---

*This requirements document serves as both a practical specification for minimal release automation and a case study in avoiding overengineering through proper requirements analysis.*