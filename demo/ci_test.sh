#!/bin/bash
# CI test script for pipz demos

set -e  # Exit on error

echo "🚀 Running CI tests for pipz demos"
echo "=================================="

# Run unit tests
echo ""
echo "📋 Unit Tests"
echo "-------------"
go test ./processors -v -short

# Run integration tests
echo ""
echo "🔗 Integration Tests"
echo "-------------------"
go test . -v -short

# Build the demo binary to ensure it compiles
echo ""
echo "🔨 Build Test"
echo "------------"
go build -o /tmp/pipz-demo .
echo "✓ Demo binary builds successfully"
rm -f /tmp/pipz-demo

# Check for race conditions
echo ""
echo "🏃 Race Detection"
echo "----------------"
go test ./... -race -short

# Run benchmarks (quick)
echo ""
echo "📊 Benchmarks"
echo "------------"
go test ./processors -bench=. -benchtime=100ms

echo ""
echo "✅ All CI tests passed!"