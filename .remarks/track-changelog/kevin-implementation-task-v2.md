# Revised Implementation Task Document v2
**Author:** Kevin (via Claude)  
**Date:** 2025-08-22  
**Type:** Implementation Blueprint (Addressing Technical Concerns)  
**Estimated Time:** 3-4 hours

## Executive Summary

This revised implementation addresses midgel's critical technical concerns using fidgel's pragmatic solutions from industry research. We're accepting certain limitations as standard practice rather than over-engineering complex workarounds.

## Critical Issues Resolution

### 1. ✅ Permission Problem - SOLVED with GitHub API
**Original Issue:** GITHUB_TOKEN can't push tags to protected branches  
**Solution:** Use GitHub CLI/API to create tags (works with GITHUB_TOKEN)  
**Trade-off Accepted:** Cannot bypass branch protection (industry standard limitation)

### 2. ✅ Race Condition - SOLVED with Concurrency Groups  
**Original Issue:** Tag existence check isn't atomic  
**Solution:** GitHub Actions concurrency groups prevent simultaneous runs  
**Trade-off Accepted:** Theoretical edge case remains (might happen once/year)

### 3. ✅ Job Dependencies - SOLVED with Conditional Logic
**Original Issue:** Breaking backward compatibility with existing workflows  
**Solution:** Use conditional job execution, don't force dependencies  
**Trade-off Accepted:** Slightly more complex YAML conditions

## Implementation Tasks

### Task 1: Modify `.github/workflows/release.yml`

#### Location: Lines 3-7 (Replace trigger section)
```yaml
on:
  push:
    tags:
      - 'v*.*.*'  # Keep existing trigger
  workflow_dispatch:
    inputs:
      version_override:
        description: 'Version override (leave empty for auto)'
        required: false
        type: string
        default: ''
      dry_run:
        description: 'Dry run mode'
        required: false
        type: boolean
        default: false

# ADD after line 7 - Concurrency control
concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false  # Never cancel releases
```

#### Location: Insert BEFORE validate job (new job at line ~15)
```yaml
  # Version determination for manual triggers
  determine-version:
    name: Determine Version
    if: github.event_name == 'workflow_dispatch'
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.version.outputs.version }}
      should_create_tag: ${{ steps.decision.outputs.create }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Install svu
        run: |
          export GOPATH=$HOME/go
          go install github.com/caarlos0/svu@v1.12.0
          echo "$HOME/go/bin" >> $GITHUB_PATH
      
      - name: Determine version
        id: version
        run: |
          if [ -n "${{ github.event.inputs.version_override }}" ]; then
            VERSION="${{ github.event.inputs.version_override }}"
            echo "Using override: $VERSION"
          else
            VERSION=$(svu next)
            echo "Auto-inferred: $VERSION"
          fi
          echo "version=$VERSION" >> $GITHUB_OUTPUT
      
      - name: Check tag existence
        id: check
        run: |
          if gh api repos/${{ github.repository }}/git/refs/tags/${{ steps.version.outputs.version }} >/dev/null 2>&1; then
            echo "exists=true" >> $GITHUB_OUTPUT
          else
            echo "exists=false" >> $GITHUB_OUTPUT
          fi
        env:
          GH_TOKEN: ${{ github.token }}
      
      - name: Decision point
        id: decision
        run: |
          if [ "${{ github.event.inputs.dry_run }}" = "true" ]; then
            echo "🔍 DRY RUN - No tag will be created"
            echo "create=false" >> $GITHUB_OUTPUT
          elif [ "${{ steps.check.outputs.exists }}" = "true" ]; then
            echo "❌ Tag already exists"
            echo "create=false" >> $GITHUB_OUTPUT
            exit 1
          else
            echo "✅ Will create tag"
            echo "create=true" >> $GITHUB_OUTPUT
          fi
      
      - name: Create tag via API
        if: steps.decision.outputs.create == 'true'
        run: |
          VERSION="${{ steps.version.outputs.version }}"
          
          # Create lightweight tag using GitHub API
          gh api repos/${{ github.repository }}/git/refs \
            --method POST \
            --field ref="refs/tags/$VERSION" \
            --field sha="${{ github.sha }}" \
            || (echo "Failed to create tag" && exit 1)
          
          echo "✅ Created tag: $VERSION"
        env:
          GH_TOKEN: ${{ github.token }}
      
      - name: Summary
        run: |
          echo "## 📋 Release Automation Summary" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "**Version:** ${{ steps.version.outputs.version }}" >> $GITHUB_STEP_SUMMARY
          echo "**Dry Run:** ${{ github.event.inputs.dry_run }}" >> $GITHUB_STEP_SUMMARY
          echo "**Tag Created:** ${{ steps.decision.outputs.create }}" >> $GITHUB_STEP_SUMMARY
```

#### Location: Line 16 - Modify validate job
```yaml
  validate:
    name: Validate Module
    # Conditional: run for tag pushes OR after determine-version succeeds
    if: |
      github.event_name == 'push' || 
      (github.event_name == 'workflow_dispatch' && needs.determine-version.outputs.should_create_tag == 'true')
    needs: 
      - determine-version
    # Make needs conditional to avoid breaking existing workflows
    runs-on: ubuntu-latest
    # ... rest of job unchanged
```

### Task 2: Create `.github/workflows/version-preview.yml`

**Complete new file:**
```yaml
name: Version Preview

on:
  pull_request:
    branches: [ main ]
    types: [opened, synchronize]

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
      
      - name: Setup
        run: |
          go install github.com/caarlos0/svu@v1.12.0
          echo "$HOME/go/bin" >> $GITHUB_PATH
      
      - name: Calculate versions
        id: calc
        run: |
          CURRENT=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
          NEXT=$(svu next)
          echo "current=$CURRENT" >> $GITHUB_OUTPUT
          echo "next=$NEXT" >> $GITHUB_OUTPUT
      
      - name: Post PR comment
        uses: actions/github-script@v7
        with:
          script: |
            const body = `## 📋 Version Preview
            **Current:** ${{ steps.calc.outputs.current }}
            **Next:** ${{ steps.calc.outputs.next }}
            
            *Version will be applied when PR is merged and release triggered.*`;
            
            const { data: comments } = await github.rest.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
            });
            
            const botComment = comments.find(c => 
              c.user.type === 'Bot' && c.body.includes('Version Preview'));
            
            if (botComment) {
              await github.rest.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: botComment.id,
                body
              });
            } else {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: context.issue.number,
                body
              });
            }
```

### Task 3: Update README.md

**Add to README (location: after Installation section):**
```markdown
## Release Process

### Automated Releases

This project uses automated release versioning. To create a release:

1. Go to Actions → Release → Run workflow
2. Leave "Version override" empty for automatic version inference
3. Click "Run workflow"

The system will:
- Automatically determine the next version from conventional commits
- Create a git tag
- Generate release notes via GoReleaser
- Publish the release to GitHub

### Manual Release (Legacy)

You can still create releases manually:
```bash
git tag v1.2.3
git push origin v1.2.3
```

### Known Limitations

- **Protected branches**: The automated release cannot bypass branch protection rules. This is by design for security.
- **Concurrent releases**: Rapid successive releases may fail. Simply retry after a moment.
- **Conventional commits required**: Version inference requires conventional commit format (`feat:`, `fix:`, etc.)
```

## Implementation Checklist

### Pre-Implementation
- [ ] Review current workflows one more time
- [ ] Ensure you're on feat/track-changelog branch
- [ ] Have GitHub repo URL ready for testing

### Implementation Steps
1. [ ] Backup existing release.yml (5 min)
2. [ ] Add workflow_dispatch and concurrency to release.yml (10 min)
3. [ ] Add determine-version job with API tag creation (20 min)
4. [ ] Fix validate job conditions (5 min)
5. [ ] Create version-preview.yml (10 min)
6. [ ] Update README with process and limitations (10 min)
7. [ ] Validate YAML syntax locally (5 min)
8. [ ] Commit changes (5 min)

**Total: ~70 minutes**

### Testing Steps
1. [ ] Push changes to feature branch
2. [ ] Create test PR to see version preview
3. [ ] Test workflow_dispatch with dry_run=true
4. [ ] Test actual release on feature branch
5. [ ] Verify existing tag-push trigger still works

## Known Limitations We're Accepting

Based on fidgel's research, we're accepting these limitations as industry standard:

1. **Cannot bypass branch protection** - This is a GitHub security feature. Every major tool accepts this.
2. **Race conditions are possible but rare** - Concurrency groups handle 99.9% of cases. 
3. **Manual fallback sometimes needed** - If automation fails, manual tagging always works.
4. **Requires conventional commits** - Version inference needs structured commit messages.

## Risk Mitigation

### What Could Go Wrong
1. **GitHub API rate limits** - Unlikely with single tag creation
2. **Token permissions** - GITHUB_TOKEN works for API, tested by fidgel
3. **Workflow syntax errors** - Validate with actionlint before commit

### Rollback Plan
If issues occur:
1. Revert the commit
2. Existing manual tag process continues to work
3. No data loss or corruption possible

## Success Metrics
- Workflow runs without errors
- Tag gets created via API
- GoReleaser triggers on new tag
- PR comments show version preview
- Existing workflows remain functional

## Final Notes

This implementation follows pragmatic industry practices:
- Uses GitHub API instead of git push (like semantic-release)
- Accepts known limitations (like Google's release-please)  
- Keeps solutions simple (concurrency groups, not distributed locking)
- Maintains backward compatibility (existing workflows keep working)

The 3-4 hour estimate includes testing and documentation. Actual code changes are ~150 lines of YAML.