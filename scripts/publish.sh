#!/bin/bash
# Script to automate GitHub tagging and release creation

set -e

# Colors for output
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${CYAN}🚀 Starting GitHub Release Automation...${NC}"

# 1. Check if gh CLI is authenticated
if ! gh auth status &>/dev/null; then
    echo -e "${RED}❌ Error: gh CLI is not authenticated. Please run 'gh auth login' first.${NC}"
    exit 1
fi

# 2. Get the new version tag
latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo -e "${YELLOW}Latest tag found: ${latest_tag}${NC}"
read -p "Enter new version tag (e.g., v1.0.0): " version

if [[ -z "$version" ]]; then
    echo -e "${RED}❌ Version tag cannot be empty.${NC}"
    exit 1
fi

# 3. Get release title
read -p "Enter release title (default: $version): " title
title=${title:-$version}

# 4. Check for pre-release
read -p "Is this a pre-release? (y/N): " is_prerelease
prerelease_flag=""
if [[ "$is_prerelease" =~ ^[Yy]$ ]]; then
    prerelease_flag="--prerelease"
    echo -e "${YELLOW}⚠️  Labeling as pre-release...${NC}"
fi

# 5. Build binaries
echo -e "${CYAN}🏗️  Building binaries for all platforms...${NC}"
make clean
make release

# 6. Create Git tag
echo -e "${CYAN}🏷️  Creating git tag $version...${NC}"
git tag -a "$version" -m "Release $version"
echo -e "${CYAN}⬆️  Pushing tag to origin...${NC}"
git push origin "$version"

# 7. Create GitHub release and upload artifacts
echo -e "${CYAN}📦 Creating GitHub release and uploading artifacts...${NC}"
gh release create "$version" dist/* \
    --title "$title" \
    --generate-notes \
    $prerelease_flag

echo -e "${GREEN}🎉 Successfully published $version to GitHub!${NC}"
echo -e "${GREEN}Check it out at: $(git remote get-url origin | sed 's/\.git$//')/releases/tag/$version${NC}"
