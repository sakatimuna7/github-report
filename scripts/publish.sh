#!/bin/bash
# Script to automate GitHub tagging and release creation

set -e

# Colors for output
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${CYAN}🚀 Starting GitHub Release Automation with AI...${NC}"

# 1. Load GROQ_API_KEY from .env (only if not in CI)
if [[ "$CI_MODE" != "true" ]]; then
    if [ -f .env ]; then
        export $(grep -v '^#' .env | xargs)
    fi
fi

if [[ -z "$GROQ_API_KEY" ]]; then
    echo -e "${RED}❌ Error: GROQ_API_KEY not found.${NC}"
    exit 1
fi

# 2. Check if gh CLI is authenticated
if [[ "$CI_MODE" == "true" ]]; then
    if [[ -z "$GITHUB_TOKEN" ]]; then
        echo -e "${RED}❌ Error: GITHUB_TOKEN not found in CI.${NC}"
        exit 1
    fi
    # In CI, gh uses GITHUB_TOKEN automatically
else
    if ! gh auth status &>/dev/null; then
        echo -e "${RED}❌ Error: gh CLI is not authenticated. Please run 'gh auth login' first.${NC}"
        exit 1
    fi
fi

# 3. Get context for AI
latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
if [[ -z "$latest_tag" ]]; then
    echo -e "${YELLOW}ℹ️  No tags found. Analyzing all commits...${NC}"
    git_log=$(git log --oneline)
    latest_tag="none"
else
    git_log=$(git log ${latest_tag}..HEAD --oneline)
fi

if [[ -z "$git_log" && -z "$INPUT_VERSION" ]]; then
    echo -e "${YELLOW}⚠️  No new commits since $latest_tag. Are you sure you want to release?${NC}"
    git_log="No new commits found (force release)"
fi

# 4. Handle Inputs (Manual vs AI)
if [[ -n "$INPUT_VERSION" ]]; then
    echo -e "${CYAN}📝 Using manual inputs from Workflow...${NC}"
    version=$INPUT_VERSION
    title=${INPUT_TITLE:-$version}
    description="Manual release from GitHub Actions"
    is_prerelease=${INPUT_PRERELEASE:-false}
else
    echo -e "${CYAN}🤖 AI is analyzing changes since ${latest_tag}...${NC}"

    prompt="You are a release manager. Based on the latest tag '$latest_tag' and the following commits, suggest the next SemVer tag, a title, and a concise summary of changes for a GitHub release.
    Commits:
    $git_log
    Return ONLY valid JSON in this format: { \"version\": \"...\", \"title\": \"...\", \"description\": \"...\", \"is_prerelease\": false }"

    payload=$(jq -n --arg prompt "$prompt" \
      '{
        model: "llama-3.3-70b-versatile",
        messages: [{role: "user", content: $prompt}],
        response_format: {type: "json_object"}
      }')

    response=$(curl -s -X POST "https://api.groq.com/openai/v1/chat/completions" \
         -H "Authorization: Bearer $GROQ_API_KEY" \
         -H "Content-Type: application/json" \
         -d "$payload")

    if echo "$response" | grep -q "error"; then
        echo -e "${RED}❌ API Error:$(echo "$response" | jq -r '.error.message')${NC}"
        exit 1
    fi

    ai_suggestion=$(echo "$response" | jq -r '.choices[0].message.content')
    version=$(echo "$ai_suggestion" | jq -r '.version')
    title=$(echo "$ai_suggestion" | jq -r '.title')
    description=$(echo "$ai_suggestion" | jq -r '.description')
    is_prerelease=$(echo "$ai_suggestion" | jq -r '.is_prerelease')
fi

# 5. Confirmation
if [[ "$CI_MODE" == "true" ]]; then
    confirm="y"
else
    echo -e "${YELLOW}--- RELEASE SETTINGS ---${NC}"
    echo -e "${CYAN}Version: ${NC}$version"
    echo -e "${CYAN}Title:   ${NC}$title"
    echo -e "${CYAN}Summary: ${NC}$description"
    echo -e "${CYAN}Pre-rel: ${NC}$is_prerelease"
    echo -e "${YELLOW}----------------------${NC}"
    read -p "Proceed with these settings? (y/n/edit): " confirm
fi

if [[ "$confirm" == "edit" ]]; then
    read -p "Enter version tag: " version
    read -p "Enter release title: " title
    read -p "Enter description: " description
elif [[ "$confirm" != "y" ]]; then
    echo -e "${RED}❌ Release cancelled.${NC}"
    exit 1
fi

prerelease_flag=""
if [ "$is_prerelease" = "true" ]; then
    prerelease_flag="--prerelease"
fi

# 6. Build binaries
echo -e "${CYAN}🏗️  Building binaries for all platforms...${NC}"
make clean
make release

# 7. Create Git tag
echo -e "${CYAN}🏷️  Creating git tag $version...${NC}"
git tag -a "$version" -m "$title"
echo -e "${CYAN}⬆️  Pushing tag to origin...${NC}"
git push origin "$version"

# 8. Create GitHub release and upload artifacts
echo -e "${CYAN}📦 Creating GitHub release and uploading artifacts...${NC}"
gh release create "$version" dist/* \
    --title "$title" \
    --notes "$description" \
    $prerelease_flag

echo -e "${GREEN}🎉 Successfully published $version to GitHub!${NC}"
echo -e "${GREEN}Check it out at: $(git remote get-url origin | sed 's/\.git$//')/releases/tag/$version${NC}"
