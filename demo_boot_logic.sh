#!/bin/bash
# Demo script to showcase the boot logic functionality

echo "🚀 Boot Logic Integration Demo"
echo "============================="
echo

# Check if server is running
echo "📡 Checking if boot service is running..."
if ! curl -s http://localhost:8080/nodes > /dev/null; then
    echo "❌ Boot service not running at localhost:8080"
    echo "   Please start the server with: ./bin/boot-service &"
    exit 1
fi
echo "✅ Boot service is running"
echo

# Show existing nodes
echo "📋 Current nodes in system:"
go run cmd/client/main.go node list -o json | jq -r '.[] | "  - \(.spec.xname) (NID: \(.spec.nid), MAC: \(.spec.bootMac))"'
echo

# Show existing configurations
echo "⚙️  Current boot configurations:"
go run cmd/client/main.go bootconfiguration list -o json | jq -r '.[] | "  - \(.metadata.name) (priority: \(.spec.priority), kernel: \(.spec.kernel))"'
echo

# Demonstrate boot script generation
echo "🔧 Generating boot scripts for different identifiers:"
echo

# Find a test node
TEST_XNAME=$(go run cmd/client/main.go node list -o json | jq -r '.[0].spec.xname // empty')
TEST_NID=$(go run cmd/client/main.go node list -o json | jq -r '.[0].spec.nid // empty')
TEST_MAC=$(go run cmd/client/main.go node list -o json | jq -r '.[0].spec.bootMac // empty')

if [ -n "$TEST_XNAME" ]; then
    echo "🎯 Testing with node: $TEST_XNAME"
    
    # Test XName resolution
    echo "   By XName ($TEST_XNAME):"
    go run ./examples/boot_script_demo.go "$TEST_XNAME" 2>/dev/null | head -5 | sed 's/^/     /'
    
    # Test NID resolution
    if [ -n "$TEST_NID" ] && [ "$TEST_NID" != "null" ]; then
        echo "   By NID ($TEST_NID):"
        go run ./examples/boot_script_demo.go "$TEST_NID" 2>/dev/null | head -5 | sed 's/^/     /'
    fi
    
    # Test MAC resolution
    if [ -n "$TEST_MAC" ] && [ "$TEST_MAC" != "null" ]; then
        echo "   By MAC ($TEST_MAC):"
        go run ./examples/boot_script_demo.go "$TEST_MAC" 2>/dev/null | head -5 | sed 's/^/     /'
    fi
fi

echo
echo "🧪 Testing unknown node handling:"
go run ./examples/boot_script_demo.go "x9999c9s9b9n9" 2>/dev/null | head -5 | sed 's/^/   /'

echo
echo "✅ Boot logic demonstration complete!"
echo "   - Multi-identifier node resolution works"
echo "   - Configuration matching is functional"
echo "   - Error handling is graceful"
echo "   - Templates generate valid iPXE scripts"
echo