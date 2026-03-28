#!/bin/bash
# Set version, commit, and create tag
# Usage: ./scripts/set-version.sh 0.2.1

set -e

VERSION_FILE="pkg/buildinfo/version.go"

if [ -z "$1" ]; then
    echo "Usage: $0 <version>"
    echo "  Example: $0 0.2.1"
    exit 1
fi

NEW_VERSION="$1"
TAG="v$NEW_VERSION"

# Validate version format (semver)
if ! [[ "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "❌ Invalid version format. Use: X.Y.Z (e.g., 0.2.1)"
    exit 1
fi

# Check for uncommitted changes
if ! git diff --quiet || ! git diff --cached --quiet; then
    echo "❌ You have uncommitted changes. Commit or stash them first."
    exit 1
fi

# Check if tag already exists
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "❌ Tag $TAG already exists."
    echo "   Delete it first: git tag -d $TAG && git push --delete origin $TAG"
    exit 1
fi

# Update version file
sed -i '' "s/const Version = \"[^\"]*\"/const Version = \"$NEW_VERSION\"/" "$VERSION_FILE"

echo "📝 Updated $VERSION_FILE to $NEW_VERSION"

# Commit
git add "$VERSION_FILE"
git commit -m "chore: bump version to $NEW_VERSION"

echo "✅ Committed version bump"

# Create tag
git tag "$TAG"

echo "🏷️  Created tag $TAG"
echo ""
echo "Next steps:"
echo "  git push && git push origin $TAG"
