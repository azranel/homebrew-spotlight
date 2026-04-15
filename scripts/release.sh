#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION=$(jq -r .version "$ROOT/package.json")
TAG="v$VERSION"
DIST="$ROOT/dist/release"

echo "==> Releasing spotlight $TAG"

# Check for uncommitted changes
if ! git -C "$ROOT" diff-index --quiet HEAD --; then
  echo "Error: uncommitted changes. Commit or stash first."
  exit 1
fi

# Check if tag already exists
if git -C "$ROOT" rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Error: tag $TAG already exists. Bump version in package.json first."
  exit 1
fi

# Check gh auth
if ! gh auth status >/dev/null 2>&1; then
  echo "Error: not authenticated with gh. Run 'gh auth login' first."
  exit 1
fi

# Clean and create dist directory
rm -rf "$DIST"
mkdir -p "$DIST"

# Build for all platforms
echo "==> Building binaries..."

NAMES=("spotlight-darwin-arm64" "spotlight-darwin-x64" "spotlight-linux-x64")
BUN_TARGETS=("bun-darwin-arm64" "bun-darwin-x64" "bun-linux-x64")

for i in 0 1 2; do
  echo "    ${BUN_TARGETS[$i]} -> ${NAMES[$i]}"
  bun build --compile --target="${BUN_TARGETS[$i]}" --outfile="$DIST/${NAMES[$i]}" "$ROOT/src/index.ts"
done

# Create tarballs and compute SHA256
echo "==> Creating tarballs..."

SHA_ARM64=""
SHA_X64=""
SHA_LINUX=""

for i in 0 1 2; do
  name="${NAMES[$i]}"
  tarball="$name.tar.gz"
  tar -czf "$DIST/$tarball" -C "$DIST" "$name"
  sha=$(shasum -a 256 "$DIST/$tarball" | awk '{print $1}')
  echo "    $tarball  sha256:$sha"

  case $i in
    0) SHA_ARM64="$sha" ;;
    1) SHA_X64="$sha" ;;
    2) SHA_LINUX="$sha" ;;
  esac
done

# Tag and push
echo "==> Tagging $TAG..."
git -C "$ROOT" tag -a "$TAG" -m "Release $TAG"
git -C "$ROOT" push origin "$TAG"

# Create GitHub release
echo "==> Creating GitHub release..."
gh release create "$TAG" \
  --repo "$(gh repo view --json nameWithOwner -q .nameWithOwner)" \
  --title "spotlight $TAG" \
  --notes "Release $TAG" \
  "$DIST/spotlight-darwin-arm64.tar.gz" \
  "$DIST/spotlight-darwin-x64.tar.gz" \
  "$DIST/spotlight-linux-x64.tar.gz"

# Update Homebrew formula with real SHA256 values
echo "==> Updating Homebrew formula..."
FORMULA="$ROOT/Formula/spotlight.rb"

# Update version and download URLs
sed -i '' "s|/download/v[^/]*/|/download/$TAG/|g" "$FORMULA"
sed -i '' "s/version \".*\"/version \"$VERSION\"/" "$FORMULA"

# Replace placeholder or previous SHA256 values
sed -i '' "/spotlight-darwin-arm64.tar.gz/{n;s/sha256 \"[^\"]*\"/sha256 \"$SHA_ARM64\"/;}" "$FORMULA"
sed -i '' "/spotlight-darwin-x64.tar.gz/{n;s/sha256 \"[^\"]*\"/sha256 \"$SHA_X64\"/;}" "$FORMULA"
sed -i '' "/spotlight-linux-x64.tar.gz/{n;s/sha256 \"[^\"]*\"/sha256 \"$SHA_LINUX\"/;}" "$FORMULA"

echo "==> Done!"
echo ""
echo "Formula updated at $FORMULA"
echo "Next steps:"
echo "  1. Review the formula:  cat $FORMULA"
echo "  2. Commit and push:    git add Formula/spotlight.rb && git commit -m 'release: update formula for $TAG' && git push"
echo "  3. If you have a separate homebrew-spotlight tap repo, copy the formula there too"
