#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 <tag>"
  echo ""
  echo "Examples:"
  echo "  $0 v0.1.0                        # main module"
  echo "  $0 modules/credentials/v0.1.0    # nested module"
  echo "  $0 modules/http/v0.1.0           # nested module"
  exit 2
}

[ $# -eq 1 ] || usage

TAG="$1"

# Validate tag format
if ! echo "$TAG" | grep -qE '^(v[0-9]+\.[0-9]+\.[0-9]+|modules/[a-z]+/v[0-9]+\.[0-9]+\.[0-9]+)$'; then
  echo "error: invalid tag format: $TAG"
  echo "Expected: vX.Y.Z or modules/<name>/vX.Y.Z"
  exit 1
fi

# Must be clean
if [ -n "$(git status --porcelain)" ]; then
  echo "error: working directory is not clean"
  git status --short
  exit 1
fi

# Tag must not exist
if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "error: tag $TAG already exists"
  exit 1
fi

# Determine module path for proxy verification
if echo "$TAG" | grep -qE '^v'; then
  MODULE="gofu.dev/gofu"
  VERSION="$TAG"
else
  # modules/credentials/v0.1.0 → gofu.dev/gofu/credentials @ v0.1.0
  MOD_NAME=$(echo "$TAG" | sed 's|^modules/||; s|/v[0-9].*||')
  VERSION=$(echo "$TAG" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+$')
  MODULE="gofu.dev/gofu/$MOD_NAME"
fi

echo "Publishing $MODULE@$VERSION"
echo ""

# Create and push tag
git tag "$TAG"
git push origin "$TAG"

echo ""
echo "Tag $TAG pushed."
echo ""

# Verify on Go proxy
PROXY_URL="https://proxy.golang.org/${MODULE}/@v/${VERSION}.info"
echo "Verifying: $PROXY_URL"
echo "(This may take a minute for the proxy to index...)"

for i in 1 2 3 4 5; do
  if curl -sf "$PROXY_URL" >/dev/null 2>&1; then
    echo "Verified on Go proxy."
    exit 0
  fi
  echo "  Attempt $i: not yet available, waiting 15s..."
  sleep 15
done

echo ""
echo "warning: module not yet visible on proxy after 75s"
echo "Check manually: $PROXY_URL"
exit 0
