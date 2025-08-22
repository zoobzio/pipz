# Implementation Analysis: Simplified Release Automation

**Author:** Kevin (via Claude)  
**Date:** 2025-08-22  
**Type:** Dry Run Analysis (No Code Changes)  
**Estimated Implementation Time:** 4-6 hours

## Executive Summary

After analyzing the existing infrastructure and requirements, the implementation requires minimal changes: adding ~50 lines of YAML to enable workflow_dispatch with automatic version inference. The existing GoReleaser setup handles everything else perfectly.

## Current State Analysis

### Existing `.github/workflows/release.yml` (170 lines)
- **Trigger:** Only on push of tags matching `v*.*.*`
- **Jobs:** validate → release → verify → notify
- **Strengths:** Comprehensive validation, GoReleaser integration, post-release verification
- **Gap:** No manual trigger mechanism, no automatic version inference

### Existing `.goreleaser.yml`
- **Changelog:** Already configured with conventional commit grouping
- **Release:** GitHub release creation with formatted notes
- **Status:** No changes needed - perfect as is

## Required Changes

### 1. Modify `.github/workflows/release.yml`

#### Location: Line 3-7 (Trigger Section)
**Current:**
```yaml
on:
  push:
    tags:
      - 'v*.*.*'
```

**Change to:**
```yaml
on:
  push:
    tags:
      - 'v*.*.*'
  workflow_dispatch:
    inputs:
      version_override:
        description: 'Version override (leave empty for auto-inference)'
        required: false
        type: string
      dry_run:
        description: 'Dry run (show version without creating tag)'
        required: false
        type: boolean
        default: false
```

#### Location: After line 13, before `validate` job
**Add Concurrency Control:**
```yaml
concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false
```

#### Location: Before `validate` job (new job)
**Add Version Inference Job:**
```yaml
  # Determine version for workflow_dispatch
  determine-version:
    name: Determine Version
    if: github.event_name == 'workflow_dispatch'
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.version.outputs.version }}
      should_proceed: ${{ steps.check.outputs.proceed }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Install svu
        run: |
          go install github.com/caarlos0/svu@v1.12.0
          echo "$(go env GOPATH)/bin" >> $GITHUB_PATH
      
      - name: Determine next version
        id: version
        run: |
          if [ -n "${{ github.event.inputs.version_override }}" ]; then
            VERSION="${{ github.event.inputs.version_override }}"
            echo "Using override version: $VERSION"
          else
            VERSION=$(svu next)
            echo "Auto-inferred version: $VERSION"
          fi
          echo "version=$VERSION" >> $GITHUB_OUTPUT
      
      - name: Check if tag exists
        id: check
        run: |
          if git rev-parse "${{ steps.version.outputs.version }}" >/dev/null 2>&1; then
            echo "❌ Tag ${{ steps.version.outputs.version }} already exists!"
            echo "proceed=false" >> $GITHUB_OUTPUT
            exit 1
          else
            echo "✅ Tag ${{ steps.version.outputs.version }} is available"
            echo "proceed=true" >> $GITHUB_OUTPUT
          fi
      
      - name: Create release preview
        run: |
          echo "## 📋 Release Preview" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "**Version:** ${{ steps.version.outputs.version }}" >> $GITHUB_STEP_SUMMARY
          echo "**Dry Run:** ${{ github.event.inputs.dry_run }}" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "### Commits since last release" >> $GITHUB_STEP_SUMMARY
          LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
          if [ -n "$LAST_TAG" ]; then
            git log --oneline $LAST_TAG..HEAD >> $GITHUB_STEP_SUMMARY
          else
            echo "This will be the first release" >> $GITHUB_STEP_SUMMARY
          fi
      
      - name: Create tag (if not dry run)
        if: github.event.inputs.dry_run != 'true'
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git tag ${{ steps.version.outputs.version }}
          git push origin ${{ steps.version.outputs.version }}
```

#### Location: Line 16 (validate job)
**Modify `validate` job dependency:**
```yaml
  validate:
    name: Validate Module
    needs: [determine-version]  # Add this dependency
    if: |
      always() && 
      (github.event_name == 'push' || 
       (github.event_name == 'workflow_dispatch' && 
        github.event.inputs.dry_run != 'true' && 
        needs.determine-version.outputs.should_proceed == 'true'))
    runs-on: ubuntu-latest
```

### 2. Create `.github/workflows/version-preview.yml`

**New file (complete):**
```yaml
name: Version Preview

on:
  pull_request:
    branches: [ main ]
    types: [opened, synchronize, reopened]

permissions:
  pull-requests: write
  contents: read

jobs:
  preview:
    name: Preview Next Version
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          ref: ${{ github.event.pull_request.head.sha }}
      
      - name: Fetch base branch
        run: |
          git fetch origin ${{ github.event.pull_request.base.ref }}
      
      - name: Install svu
        run: |
          go install github.com/caarlos0/svu@v1.12.0
          echo "$(go env GOPATH)/bin" >> $GITHUB_PATH
      
      - name: Get current version
        id: current
        run: |
          CURRENT=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
          echo "version=$CURRENT" >> $GITHUB_OUTPUT
      
      - name: Preview next version
        id: next
        run: |
          # Get commits in this PR
          BASE_SHA=$(git merge-base HEAD origin/${{ github.event.pull_request.base.ref }})
          
          # Temporarily create a lightweight tag at base to help svu
          git tag temp-base $BASE_SHA
          
          # Get what version would be
          NEXT=$(svu next)
          
          # Clean up temp tag
          git tag -d temp-base
          
          echo "version=$NEXT" >> $GITHUB_OUTPUT
      
      - name: Analyze commits
        id: analysis
        run: |
          BASE_SHA=$(git merge-base HEAD origin/${{ github.event.pull_request.base.ref }})
          
          # Count different types of commits
          FEATURES=$(git log --oneline $BASE_SHA..HEAD | grep -c "^feat" || true)
          FIXES=$(git log --oneline $BASE_SHA..HEAD | grep -c "^fix" || true)
          BREAKING=$(git log --oneline $BASE_SHA..HEAD | grep -c "!" || true)
          
          echo "features=$FEATURES" >> $GITHUB_OUTPUT
          echo "fixes=$FIXES" >> $GITHUB_OUTPUT
          echo "breaking=$BREAKING" >> $GITHUB_OUTPUT
      
      - name: Comment on PR
        uses: actions/github-script@v7
        with:
          script: |
            const current = '${{ steps.current.outputs.version }}';
            const next = '${{ steps.next.outputs.version }}';
            const features = ${{ steps.analysis.outputs.features }};
            const fixes = ${{ steps.analysis.outputs.fixes }};
            const breaking = ${{ steps.analysis.outputs.breaking }};
            
            const body = `## 📋 Version Preview
            
            **Current Version:** ${current}
            **Next Version:** ${next}
            
            ### Changes in this PR
            - ✨ Features: ${features}
            - 🐛 Fixes: ${fixes}
            - ⚠️ Breaking: ${breaking}
            
            ---
            *This version will be applied when the PR is merged and a release is triggered.*`;
            
            // Find existing comment
            const { data: comments } = await github.rest.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
            });
            
            const botComment = comments.find(comment => 
              comment.user.type === 'Bot' && 
              comment.body.includes('## 📋 Version Preview')
            );
            
            if (botComment) {
              await github.rest.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: botComment.id,
                body: body
              });
            } else {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: context.issue.number,
                body: body
              });
            }
```

## Implementation Checklist

### Pre-Implementation
- [ ] Backup current workflows
- [ ] Review GitHub token permissions
- [ ] Verify branch protection rules allow bot commits

### Implementation Steps
1. [ ] Add workflow_dispatch to release.yml (10 min)
2. [ ] Add concurrency control (5 min)
3. [ ] Add determine-version job (20 min)
4. [ ] Update validate job conditions (5 min)
5. [ ] Create version-preview.yml (15 min)
6. [ ] Test workflow syntax with `actionlint` (10 min)
7. [ ] Commit changes to feat/track-changelog branch (5 min)

### Testing Steps
1. [ ] Test PR preview on a draft PR
2. [ ] Test workflow_dispatch with dry_run=true
3. [ ] Test version inference without override
4. [ ] Test with version override
5. [ ] Test tag collision detection
6. [ ] Verify GoReleaser still works with manual tags

### Post-Implementation
- [ ] Document new release process in README
- [ ] Update CONTRIBUTING.md
- [ ] Create release runbook

## Risk Assessment

### Low Risk Items
- **Workflow syntax errors**: Can be caught with actionlint
- **svu installation**: Pinned version, reliable tool
- **PR comments**: Non-critical feature, can fail gracefully

### Medium Risk Items
- **Tag creation permissions**: GITHUB_TOKEN might need adjustment
- **Concurrent releases**: Mitigated with concurrency group
- **Version inference accuracy**: svu is battle-tested, but should monitor

### High Risk Items
- **Breaking existing release flow**: Mitigated by keeping push trigger intact
- **Accidental releases**: Mitigated with dry_run option and confirmation steps

## Rollback Plan

If issues occur:
1. **Immediate**: Set dry_run=true by default
2. **Short term**: Revert workflow changes (git revert)
3. **Continue**: Manual tag creation still works as before

## Time Estimate Breakdown

- **Workflow modifications**: 1 hour
- **Testing locally**: 1 hour  
- **GitHub testing**: 1-2 hours
- **Documentation**: 30 minutes
- **Buffer for issues**: 1-2 hours

**Total: 4.5-6.5 hours**

## Validation Commands

```bash
# Validate workflow syntax
actionlint .github/workflows/release.yml
actionlint .github/workflows/version-preview.yml

# Test svu locally
go install github.com/caarlos0/svu@v1.12.0
svu current
svu next

# Simulate git operations
git tag -l
git describe --tags --abbrev=0
```

## Key Success Factors

1. **Minimal changes**: Only touching workflows, not application code
2. **Backward compatible**: Existing tag push trigger remains
3. **Escape hatches**: dry_run and version_override provide control
4. **Clear feedback**: Step summaries and PR comments show what will happen

## Concerns and Recommendations

### Concerns
1. **svu version pinning**: Should pin to specific version for reproducibility
2. **Token permissions**: May need PAT for protected branches
3. **PR base branch**: Need to handle PRs not targeting main

### Recommendations
1. Test in a fork first for safety
2. Add notification to Slack/Discord on release
3. Consider adding changelog preview to workflow summary
4. Add metrics tracking for release frequency

## Conclusion

The implementation is straightforward with minimal risk. The existing infrastructure is solid, and we're only adding the missing automation layer. The 4-6 hour estimate is realistic, with most time spent on testing rather than implementation.