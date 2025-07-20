#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo -e "${RED}Error: You must be on the main branch to create a release${NC}"
    exit 1
fi

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo -e "${RED}Error: You have uncommitted changes. Please commit or stash them first.${NC}"
    exit 1
fi

# Get the latest tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo -e "${GREEN}Latest tag: $LATEST_TAG${NC}"

# Function to increment version
increment_version() {
    local version=$1
    local part=$2
    
    # Remove 'v' prefix if present
    version=${version#v}
    
    # Split version into parts
    IFS='.' read -r major minor patch <<< "$version"
    
    case $part in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
    esac
    
    echo "v${major}.${minor}.${patch}"
}

# Determine version bump type
echo -e "\n${YELLOW}What type of release is this?${NC}"
echo "1) Major (breaking changes)"
echo "2) Minor (new features)"
echo "3) Patch (bug fixes)"
read -p "Select [1-3]: " choice

case $choice in
    1) BUMP_TYPE="major" ;;
    2) BUMP_TYPE="minor" ;;
    3) BUMP_TYPE="patch" ;;
    *) echo -e "${RED}Invalid choice${NC}"; exit 1 ;;
esac

# Calculate new version
NEW_VERSION=$(increment_version "$LATEST_TAG" "$BUMP_TYPE")
echo -e "\n${GREEN}New version will be: $NEW_VERSION${NC}"

# Get commits since last tag
echo -e "\n${YELLOW}Commits since $LATEST_TAG:${NC}"
git log --oneline "$LATEST_TAG"..HEAD

# Confirm release
echo -e "\n${YELLOW}Do you want to proceed with release $NEW_VERSION? (y/n)${NC}"
read -p "> " confirm
if [ "$confirm" != "y" ]; then
    echo "Release cancelled"
    exit 0
fi

# Generate changelog entry
CHANGELOG_FILE="CHANGELOG.md"
TEMP_CHANGELOG=$(mktemp)

# Create changelog header
echo "## [$NEW_VERSION] - $(date +%Y-%m-%d)" > "$TEMP_CHANGELOG"
echo "" >> "$TEMP_CHANGELOG"

# Group commits by type
echo "### Added" >> "$TEMP_CHANGELOG"
git log --pretty=format:"- %s (%h)" "$LATEST_TAG"..HEAD --grep="^feat" >> "$TEMP_CHANGELOG" || true
echo -e "\n" >> "$TEMP_CHANGELOG"

echo "### Fixed" >> "$TEMP_CHANGELOG"
git log --pretty=format:"- %s (%h)" "$LATEST_TAG"..HEAD --grep="^fix" >> "$TEMP_CHANGELOG" || true
echo -e "\n" >> "$TEMP_CHANGELOG"

echo "### Changed" >> "$TEMP_CHANGELOG"
git log --pretty=format:"- %s (%h)" "$LATEST_TAG"..HEAD --grep="^(refactor|perf|style)" -E >> "$TEMP_CHANGELOG" || true
echo -e "\n" >> "$TEMP_CHANGELOG"

echo "### Other" >> "$TEMP_CHANGELOG"
git log --pretty=format:"- %s (%h)" "$LATEST_TAG"..HEAD --invert-grep --grep="^(feat|fix|refactor|perf|style|docs|test|chore)" -E >> "$TEMP_CHANGELOG" || true
echo -e "\n" >> "$TEMP_CHANGELOG"

# If CHANGELOG.md exists, append to it, otherwise create it
if [ -f "$CHANGELOG_FILE" ]; then
    # Insert new changelog at the beginning (after the title)
    {
        head -n 2 "$CHANGELOG_FILE" 2>/dev/null || echo "# Changelog"
        echo ""
        cat "$TEMP_CHANGELOG"
        echo ""
        tail -n +3 "$CHANGELOG_FILE" 2>/dev/null || true
    } > "${CHANGELOG_FILE}.tmp"
    mv "${CHANGELOG_FILE}.tmp" "$CHANGELOG_FILE"
else
    echo "# Changelog" > "$CHANGELOG_FILE"
    echo "" >> "$CHANGELOG_FILE"
    cat "$TEMP_CHANGELOG" >> "$CHANGELOG_FILE"
fi

rm "$TEMP_CHANGELOG"

# Stage changelog
git add "$CHANGELOG_FILE"

# Create release commit
git commit -m "chore(release): $NEW_VERSION"

# Create annotated tag
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"

echo -e "\n${GREEN}Release $NEW_VERSION created successfully!${NC}"
echo -e "\n${YELLOW}To push the release, run:${NC}"
echo "  git push origin main"
echo "  git push origin $NEW_VERSION"
echo -e "\n${YELLOW}Then create a GitHub release at:${NC}"
echo "  https://github.com/$(git remote get-url origin | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/releases/new?tag=$NEW_VERSION"