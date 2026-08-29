#!/usr/bin/env bash
set -e

echo "🚀 Building gomake..."
go build -o gomake main.go

if [ -f "./gomake" ]; then
    echo "✅ Build successful!"
    echo "👉 You can now run: ./gomake genconfig"
    echo "💡 Tip: To install globally, run: sudo cp gomake /usr/local/bin/"
else
    echo "❌ Build failed."
    exit 1
fi
