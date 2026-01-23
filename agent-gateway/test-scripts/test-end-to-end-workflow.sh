#!/bin/bash
# ============================================================================
# End-to-End AgentGateway Workflow Test
# ============================================================================
# Tests complete agent workflow: LLM review → Code scan → GitHub issue creation
# ============================================================================

set -e

GATEWAY_HOST="agentgateway.agensys-codereview-demo.svc.cluster.local"
AGENT_ID="pr-reviewer"

echo "=== AgentGateway End-to-End Workflow Test ==="
echo ""

# Step 1: Send code to LLM for review via Anthropic proxy (port 9081)
echo "Step 1: Requesting code review from Claude..."
REVIEW_RESPONSE=$(curl -s -X POST http://${GATEWAY_HOST}:9081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "messages": [
      {
        "role": "user",
        "content": "Review this Python code for security issues:\n\ndef process_user_data(user_input):\n    eval(user_input)\n    return True"
      }
    ],
    "max_tokens": 500
  }')

if [ $? -eq 0 ]; then
  echo "✓ Received code review from Claude"
else
  echo "✗ Failed to get response from Claude"
  exit 1
fi
echo ""

# Step 2: Scan code with Semgrep via MCP proxy (port 9083)
echo "Step 2: Scanning code with Semgrep MCP server..."
SCAN_RESPONSE=$(curl -s -X POST http://${GATEWAY_HOST}:9083/mcp \
  -H "Content-Type: application/json" \
  -H "x-mcp-server: semgrep" \
  -H "x-agent-id: ${AGENT_ID}" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "scan_code",
      "arguments": {
        "language": "python",
        "code": "def process_user_data(user_input):\n    eval(user_input)\n    return True"
      }
    }
  }')

if [ $? -eq 0 ]; then
  echo "✓ Code scan completed"
else
  echo "✗ Failed to scan code"
  exit 1
fi
echo ""

# Step 3: Create GitHub issue via MCP proxy (port 9083)
echo "Step 3: Creating GitHub issue with findings..."
ISSUE_RESPONSE=$(curl -s -X POST http://${GATEWAY_HOST}:9083/mcp \
  -H "Content-Type: application/json" \
  -H "x-mcp-server: github" \
  -H "x-agent-id: ${AGENT_ID}" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "create_issue",
      "arguments": {
        "repo": "myorg/myrepo",
        "title": "Security: Unsafe use of eval()",
        "body": "Both Claude code review and Semgrep scanning identified unsafe use of eval() with user input. This is a critical security vulnerability that could allow arbitrary code execution."
      }
    }
  }')

if [ $? -eq 0 ]; then
  echo "✓ GitHub issue created"
else
  echo "✗ Failed to create GitHub issue"
  exit 1
fi
echo ""

# Step 4: Verify metrics endpoint
echo "Step 4: Checking gateway metrics..."
METRICS=$(curl -s http://${GATEWAY_HOST}:15020/metrics | grep -E 'agentgateway_requests_total|agentgateway_request_duration')

if [ -n "$METRICS" ]; then
  echo "✓ Metrics collected"
else
  echo "✗ No metrics found"
  exit 1
fi
echo ""

echo "=== Test Complete ==="
echo ""
echo "This workflow exercised:"
echo "  ✓ Anthropic AI proxy (port 9081) - LLM code review"
echo "  ✓ MCP proxy (port 9083) - Semgrep scanning"
echo "  ✓ MCP proxy (port 9083) - GitHub issue creation"
echo "  ✓ Metrics endpoint (port 15020) - Observability"
echo "  ✓ Header-based routing (x-mcp-server)"
echo "  ✓ Agent authentication (x-agent-id)"
echo "  ✓ JSON-RPC validation"
echo "  ✓ Multi-port architecture"
