#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION=$(grep -o 'version = "[^"]*"' "$ROOT/cmd/root.go" | head -1 | cut -d'"' -f2)

if [ "$VERSION" = "dev" ]; then
  echo "Error: version is still 'dev'. Set it in cmd/root.go first."
  exit 1
fi

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
  echo "Error: tag $TAG already exists. Bump version in cmd/root.go first."
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

PLATFORMS=(
  "darwin:arm64"
  "darwin:amd64"
  "linux:amd64"
)

LDFLAGS="-s -w -X github.com/azranel/spotlight/cmd.version=$VERSION"

SHA_DARWIN_ARM64=""
SHA_DARWIN_AMD64=""
SHA_LINUX_AMD64=""

for i in 0 1 2; do
  platform="${PLATFORMS[$i]}"
  goos="${platform%%:*}"
  goarch="${platform##*:}"
  name="spotlight-${goos}-${goarch}"
  echo "    $goos/$goarch -> $name"
  GOOS="$goos" GOARCH="$goarch" go build -ldflags "$LDFLAGS" -o "$DIST/$name" .

  # Create tarball with binary named "spotlight"
  cp "$DIST/$name" "$DIST/spotlight"
  tar -czf "$DIST/$name.tar.gz" -C "$DIST" "spotlight"
  rm "$DIST/spotlight"
  sha=$(shasum -a 256 "$DIST/$name.tar.gz" | awk '{print $1}')
  echo "    $name.tar.gz  sha256:$sha"

  case $i in
    0) SHA_DARWIN_ARM64="$sha" ;;
    1) SHA_DARWIN_AMD64="$sha" ;;
    2) SHA_LINUX_AMD64="$sha" ;;
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
  "$DIST/spotlight-darwin-amd64.tar.gz" \
  "$DIST/spotlight-linux-amd64.tar.gz"

# Update Homebrew formula
echo "==> Updating Homebrew formula..."
FORMULA="$ROOT/Formula/spotlight.rb"

sed -i '' "s|/download/v[^/]*/|/download/$TAG/|g" "$FORMULA"
sed -i '' "s/version \".*\"/version \"$VERSION\"/" "$FORMULA"
sed -i '' "/spotlight-darwin-arm64.tar.gz/{n;s/sha256 \"[^\"]*\"/sha256 \"$SHA_DARWIN_ARM64\"/;}" "$FORMULA"
sed -i '' "/spotlight-darwin-amd64.tar.gz/{n;s/sha256 \"[^\"]*\"/sha256 \"$SHA_DARWIN_AMD64\"/;}" "$FORMULA"
sed -i '' "/spotlight-linux-amd64.tar.gz/{n;s/sha256 \"[^\"]*\"/sha256 \"$SHA_LINUX_AMD64\"/;}" "$FORMULA"

echo "==> Done!"
echo ""
echo "Formula updated at $FORMULA"
echo "Binary sizes:"
ls -lh "$DIST"/spotlight-*  | grep -v tar | awk '{print "  " $5 "  " $9}'
echo ""
echo "Next steps:"
echo "  1. Review the formula:  cat $FORMULA"
echo "  2. Commit and push:    git add Formula/spotlight.rb && git commit -m 'release: update formula for $TAG' && git push"
echo "  3. If you have a separate homebrew-spotlight tap repo, copy the formula there too"
