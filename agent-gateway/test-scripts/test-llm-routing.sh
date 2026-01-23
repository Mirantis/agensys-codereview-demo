#!/bin/bash
# ============================================================================
# LLM Routing Test
# ============================================================================
# Tests routing to Anthropic and OpenAI APIs with automatic key injection
# ============================================================================

set -e

GATEWAY_HOST="agentgateway.agensys-codereview-demo.svc.cluster.local"
PASS=0
FAIL=0

echo "=== AgentGateway LLM Routing Tests ==="
echo ""

# Test 1: Anthropic Claude API Routing
echo "Test 1: Anthropic Claude API routing (port 9081)..."
RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/anthropic_response.json -X POST http://${GATEWAY_HOST}:9081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "messages": [
      {
        "role": "user",
        "content": "Say hello in one word"
      }
    ],
    "max_tokens": 10
  }')

if [ "$RESPONSE" = "200" ]; then
  echo "✓ Anthropic routing successful"
  ((PASS++))
else
  echo "✗ Anthropic routing failed - HTTP $RESPONSE"
  cat /tmp/anthropic_response.json
  ((FAIL++))
fi
echo ""

# Test 2: OpenAI GPT API Routing
echo "Test 2: OpenAI GPT API routing (port 9082)..."
RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/openai_response.json -X POST http://${GATEWAY_HOST}:9082/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {
        "role": "user",
        "content": "Say hello in one word"
      }
    ],
    "max_tokens": 10
  }')

if [ "$RESPONSE" = "200" ]; then
  echo "✓ OpenAI routing successful"
  ((PASS++))
else
  echo "✗ OpenAI routing failed - HTTP $RESPONSE"
  cat /tmp/openai_response.json
  ((FAIL++))
fi
echo ""

# Test 3: API Key Auto-Injection (Anthropic)
echo "Test 3: API key auto-injection verification..."
# If we got 200 responses above, keys were injected correctly
if [ $PASS -eq 2 ]; then
  echo "✓ API keys auto-injected successfully (both providers responded)"
  ((PASS++))
else
  echo "✗ API key injection may have failed"
  ((FAIL++))
fi
echo ""

# Test 4: Model-Specific Routing
echo "Test 4: Model-specific routing..."
# Test different model
RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null -X POST http://${GATEWAY_HOST}:9081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Test"}],
    "max_tokens": 5
  }')

if [ "$RESPONSE" = "200" ]; then
  echo "✓ Alternative model routing successful"
  ((PASS++))
else
  echo "✗ Alternative model routing failed - HTTP $RESPONSE"
  ((FAIL++))
fi
echo ""

# Test 5: Error Handling - Invalid Model
echo "Test 5: Error handling for invalid model..."
RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null -X POST http://${GATEWAY_HOST}:9081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "invalid-model-name",
    "messages": [{"role": "user", "content": "Test"}],
    "max_tokens": 5
  }')

if [ "$RESPONSE" != "200" ]; then
  echo "✓ Invalid model correctly rejected - HTTP $RESPONSE"
  ((PASS++))
else
  echo "✗ Invalid model was accepted"
  ((FAIL++))
fi
echo ""

# Test 6: Streaming Support (if enabled)
echo "Test 6: Streaming support test..."
RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/stream_response.txt -X POST http://${GATEWAY_HOST}:9081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "messages": [{"role": "user", "content": "Count to 3"}],
    "max_tokens": 20,
    "stream": true
  }')

if [ "$RESPONSE" = "200" ]; then
  echo "✓ Streaming request handled"
  ((PASS++))
else
  echo "✗ Streaming request failed - HTTP $RESPONSE"
  ((FAIL++))
fi
echo ""

echo "=== Test Results ==="
echo "Passed: $PASS"
echo "Failed: $FAIL"
echo ""

if [ $FAIL -eq 0 ]; then
  echo "✓ All LLM routing tests passed!"
  exit 0
else
  echo "✗ Some tests failed"
  exit 1
fi
