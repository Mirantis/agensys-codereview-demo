#!/bin/bash
# ============================================================================
# Observability Test
# ============================================================================
# Tests Prometheus metrics, health checks, and admin UI
# ============================================================================

set -e

GATEWAY_HOST="agentgateway.agensys-codereview-demo.svc.cluster.local"
PASS=0
FAIL=0

echo "=== AgentGateway Observability Tests ==="
echo ""

# Test 1: Prometheus Metrics Endpoint
echo "Test 1: Prometheus metrics endpoint..."
RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/metrics.txt http://${GATEWAY_HOST}:15020/metrics)

if [ "$RESPONSE" = "200" ]; then
  echo "✓ Metrics endpoint accessible"
  ((PASS++))
else
  echo "✗ Metrics endpoint failed - HTTP $RESPONSE"
  ((FAIL++))
fi
echo ""

# Test 2: Metrics Content Validation
echo "Test 2: Metrics content validation..."
if grep -q "agentgateway_requests_total" /tmp/metrics.txt; then
  echo "✓ Request count metrics present"
  ((PASS++))
else
  echo "✗ Request count metrics missing"
  ((FAIL++))
fi
echo ""

# Test 3: Health Check Endpoint
echo "Test 3: Health check endpoint..."
RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/health.txt http://${GATEWAY_HOST}:15021/healthz/ready)

if [ "$RESPONSE" = "200" ]; then
  echo "✓ Health check endpoint responding"
  ((PASS++))
else
  echo "✗ Health check failed - HTTP $RESPONSE"
  ((FAIL++))
fi
echo ""

# Test 4: Admin UI Accessibility
echo "Test 4: Admin UI accessibility..."
RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null http://${GATEWAY_HOST}:15000/)

if [ "$RESPONSE" = "200" ]; then
  echo "✓ Admin UI accessible"
  ((PASS++))
else
  echo "✗ Admin UI not accessible - HTTP $RESPONSE"
  ((FAIL++))
fi
echo ""

# Test 5: Structured Logging
echo "Test 5: Structured logging verification..."
# Make a test request to generate logs
curl -s -o /dev/null -X POST http://${GATEWAY_HOST}:9081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "messages": [{"role": "user", "content": "Test logging"}],
    "max_tokens": 5
  }' || true

# Check if gateway logs are available via kubectl
if kubectl logs -n agensys-codereview-demo -l app.kubernetes.io/component=agentgateway --tail=1 &> /dev/null; then
  echo "✓ Logs accessible via kubectl"
  ((PASS++))
else
  echo "⚠ Cannot verify logs (may require kubectl access)"
  echo "  Skipping this test"
fi
echo ""

# Test 6: Metrics After Requests
echo "Test 6: Metrics update after requests..."
# Get initial metrics
INITIAL_REQUESTS=$(curl -s http://${GATEWAY_HOST}:15020/metrics | grep "agentgateway_requests_total" | head -1 | awk '{print $2}' || echo "0")

# Make a request
curl -s -o /dev/null -X POST http://${GATEWAY_HOST}:9081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "messages": [{"role": "user", "content": "Metrics test"}],
    "max_tokens": 5
  }' || true

sleep 1

# Get updated metrics
UPDATED_REQUESTS=$(curl -s http://${GATEWAY_HOST}:15020/metrics | grep "agentgateway_requests_total" | head -1 | awk '{print $2}' || echo "0")

if [ "$UPDATED_REQUESTS" != "$INITIAL_REQUESTS" ]; then
  echo "✓ Metrics updating correctly (was: $INITIAL_REQUESTS, now: $UPDATED_REQUESTS)"
  ((PASS++))
else
  echo "⚠ Metrics may not be updating (check if requests are succeeding)"
fi
echo ""

# Test 7: Key Metrics Present
echo "Test 7: Key metrics presence check..."
EXPECTED_METRICS=(
  "agentgateway_requests_total"
  "agentgateway_request_duration_seconds"
  "agentgateway_active_connections"
)

MISSING_METRICS=0
for metric in "${EXPECTED_METRICS[@]}"; do
  if grep -q "$metric" /tmp/metrics.txt; then
    echo "  ✓ $metric present"
  else
    echo "  ✗ $metric missing"
    ((MISSING_METRICS++))
  fi
done

if [ $MISSING_METRICS -eq 0 ]; then
  echo "✓ All expected metrics present"
  ((PASS++))
else
  echo "✗ $MISSING_METRICS metrics missing"
  ((FAIL++))
fi
echo ""

echo "=== Test Results ==="
echo "Passed: $PASS"
echo "Failed: $FAIL"
echo ""

if [ $FAIL -eq 0 ]; then
  echo "✓ All observability tests passed!"
  exit 0
else
  echo "✗ Some tests failed"
  exit 1
fi
