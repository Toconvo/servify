#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SDK_DIR="$ROOT_DIR/sdk"
DEMO_SDK_DIR="$ROOT_DIR/apps/demo-web/sdk"

echo "🔧 Building SDK packages..."
npm -C "$SDK_DIR" ci
npm -C "$SDK_DIR" run build

echo "📦 Syncing SDK artifacts to demo..."
mkdir -p "$DEMO_SDK_DIR"

VANILLA_DIST="$SDK_DIR/packages/vanilla/dist"

# Copy and rename bundles
cp -f "$VANILLA_DIST/index.esm.js" "$DEMO_SDK_DIR/servify-sdk.esm.js"
cp -f "$VANILLA_DIST/index.js" "$DEMO_SDK_DIR/servify-sdk.umd.js"

# Types
if [ -f "$VANILLA_DIST/index.d.ts" ]; then
  cp -f "$VANILLA_DIST/index.d.ts" "$DEMO_SDK_DIR/index.d.ts"
fi

echo "✅ SDK synced to apps/demo-web/sdk"
