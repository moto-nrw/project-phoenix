#!/usr/bin/env bash
set -e

echo "📦 Installing git hooks..."
cp scripts/hooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
echo "✅ Git hooks installed successfully!"
