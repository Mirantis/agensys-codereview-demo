#!/bin/bash
# ============================================================================
# MCP Routing Test
# ============================================================================
# Tests MCP server routing, JSON-RPC validation, and header-based routing
# ============================================================================

set -e

GATEWAY_HOST="agentgateway.agensys-codereview-demo.svc.cluster.local"
AGENT_ID="pr-reviewer"
PASS=0
FAIL=0

echo "=== AgentGateway MCP Routing Tests ==="
echo ""

# Test 1: GitHub MCP Server Routing
echo "Test 1: GitHub MCP server routing..."
RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/github_mcp_response.json -X POST http://${GATEWAY_HOST}:9083/mcp \
  -H "Content-Type: application/json" \
  -H "x-mcp-server: github" \
  -H "x-agent-id: ${AGENT_ID}" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list",
    "params": {}
  }')

if [ "$RESPONSE" = "200" ]; then
  echo "✓ GitHub MCP routing successful"
  ((PASS++))
else
  echo "✗ GitHub MCP routing failed - HTTP $RESPONSE"
  cat /tmp/github_mcp_response.json
  ((FAIL++))
fi
echo ""

# Test 2: Semgrep MCP Server Routing
echo "Test 2: Semgrep MCP server routing..."
RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/semgrep_mcp_response.json -X POST http://${GATEWAY_HOST}:9083/mcp \
  -H "Content-Type: application/json" \
  -H "x-mcp-server: semgrep" \
  -H "x-agent-id: ${AGENT_ID}" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }')

if [ "$RESPONSE" = "200" ]; then
  echo "✓ Semgrep MCP routing successful"
  ((PASS++))
else
  echo "✗ Semgrep MCP routing failed - HTTP $RESPONSE"
  cat /tmp/semgrep_mcp_response.json
  ((FAIL++))
fi
echo ""

# Test 3: JSON-RPC Validation
echo "Test 3: JSON-RPC format validation..."
RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null -X POST http://${GATEWAY_HOST}:9083/mcp \
  -H "Content-Type: application/json" \
  -H "x-mcp-server: github" \
  -H "x-agent-id: ${AGENT_ID}" \
  -d '{
    "invalid": "format",
    "no_jsonrpc": "field"
  }')

if [ "$RESPONSE" = "403" ] || [ "$RESPONSE" = "400" ]; then
  echo "✓ Invalid JSON-RPC correctly rejected"
  ((PASS++))
else
  echo "✗ Invalid JSON-RPC was accepted - HTTP $RESPONSE"
  ((FAIL++))
fi
echo ""

# Test 4: Header-Based Routing (missing x-mcp-server)
echo "Test 4: Header-based routing validation..."
RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null -X POST http://${GATEWAY_HOST}:9083/mcp \
  -H "Content-Type: application/json" \
  -H "x-agent-id: ${AGENT_ID}" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/list",
    "params": {}
  }')

if [ "$RESPONSE" != "200" ]; then
  echo "✓ Missing x-mcp-server header correctly rejected"
  ((PASS++))
else
  echo "✗ Request without x-mcp-server was accepted"
  ((FAIL++))
fi
echo ""

# Test 5: Header-Based Routing (invalid server)
echo "Test 5: Invalid MCP server name..."
RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null -X POST http://${GATEWAY_HOST}:9083/mcp \
  -H "Content-Type: application/json" \
  -H "x-mcp-server: invalid-server" \
  -H "x-agent-id: ${AGENT_ID}" \
  -d '{
    "jsonrpc": "2.0",
    "id": 4,
    "method": "tools/list",
    "params": {}
  }')

if [ "$RESPONSE" != "200" ]; then
  echo "✓ Invalid server name correctly rejected"
  ((PASS++))
else
  echo "✗ Invalid server name was accepted"
  ((FAIL++))
fi
echo ""

# Test 6: MCP Tool Call
echo "Test 6: MCP tool call..."
RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/mcp_tool_response.json -X POST http://${GATEWAY_HOST}:9083/mcp \
  -H "Content-Type: application/json" \
  -H "x-mcp-server: github" \
  -H "x-agent-id: ${AGENT_ID}" \
  -d '{
    "jsonrpc": "2.0",
    "id": 5,
    "method": "tools/call",
    "params": {
      "name": "list_repositories",
      "arguments": {
        "org": "example-org"
      }
    }
  }')

if [ "$RESPONSE" = "200" ]; then
  echo "✓ MCP tool call successful"
  ((PASS++))
else
  echo "✗ MCP tool call failed - HTTP $RESPONSE"
  cat /tmp/mcp_tool_response.json
  ((FAIL++))
fi
echo ""

# Test 7: Per-Server Rate Limits
echo "Test 7: Per-server rate limits (GitHub: 60/min, Semgrep: 30/min)..."
# Send burst to GitHub
GITHUB_SUCCESS=0
for i in {1..10}; do
  RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null -X POST http://${GATEWAY_HOST}:9083/mcp \
    -H "Content-Type: application/json" \
    -H "x-mcp-server: github" \
    -H "x-agent-id: ${AGENT_ID}" \
    -d '{
      "jsonrpc": "2.0",
      "id": '$i',
      "method": "tools/list",
      "params": {}
    }')
  
  if [ "$RESPONSE" = "200" ]; then
    ((GITHUB_SUCCESS++))
  fi
  sleep 0.1
done

if [ $GITHUB_SUCCESS -ge 8 ]; then
  echo "✓ GitHub MCP rate limit appropriate ($GITHUB_SUCCESS/10 succeeded)"
  ((PASS++))
else
  echo "✗ GitHub MCP rate limit too restrictive ($GITHUB_SUCCESS/10 succeeded)"
  ((FAIL++))
fi
echo ""

echo "=== Test Results ==="
echo "Passed: $PASS"
echo "Failed: $FAIL"
echo ""

if [ $FAIL -eq 0 ]; then
  echo "✓ All MCP routing tests passed!"
  exit 0
else
  echo "✗ Some tests failed"
  exit 1
fi
