# AgentGateway Test Scripts

Comprehensive test suite for validating AgentGateway deployment and functionality.

## Quick Start

```bash
# Run all tests
./run-all-tests.sh

# Or run individual test suites
./test-security-policies.sh
./test-llm-routing.sh
./test-mcp-routing.sh
./test-observability.sh
./test-end-to-end-workflow.sh
```

## Test Suites

### 1. Security Policy Tests (`test-security-policies.sh`)

Validates security policies and protections:
- ✅ Secret filtering (blocks API keys in requests)
- ✅ Prompt injection detection
- ✅ Valid request handling
- ✅ Agent ID validation
- ✅ JSON-RPC format validation
- ✅ Rate limiting enforcement

**Expected:** 6/6 tests pass

### 2. LLM Routing Tests (`test-llm-routing.sh`)

Tests AI provider integrations:
- ✅ Anthropic Claude API routing
- ✅ OpenAI GPT API routing
- ✅ Automatic API key injection
- ✅ Model-specific routing
- ✅ Error handling for invalid models
- ✅ Streaming support

**Expected:** 6/6 tests pass

### 3. MCP Routing Tests (`test-mcp-routing.sh`)

Tests MCP server routing (requires MCP servers deployed):
- ✅ GitHub MCP server routing
- ✅ Semgrep MCP server routing
- ✅ JSON-RPC validation
- ✅ Header-based routing
- ✅ Invalid server name rejection
- ✅ MCP tool calls
- ✅ Per-server rate limits

**Expected:** 7/7 tests pass  
**Requires:** `kubectl apply -f ../06-mcp-servers.yaml`

### 4. Observability Tests (`test-observability.sh`)

Validates monitoring and metrics:
- ✅ Prometheus metrics endpoint
- ✅ Metrics content validation
- ✅ Health check endpoint
- ✅ Admin UI accessibility
- ✅ Structured logging
- ✅ Metrics update after requests
- ✅ Key metrics presence

**Expected:** 7/7 tests pass

### 5. End-to-End Workflow Test (`test-end-to-end-workflow.sh`)

Tests complete agent workflow (requires MCP servers):
- ✅ LLM code review via Anthropic
- ✅ Code scanning via Semgrep MCP
- ✅ GitHub issue creation via GitHub MCP
- ✅ Metrics collection

**Expected:** All steps complete successfully  
**Requires:** `kubectl apply -f ../06-mcp-servers.yaml`

## Prerequisites

### Required
1. AgentGateway deployed: `kubectl apply -f ../manifests/`
2. API secrets configured with valid keys in `02-secrets.yaml`
3. `curl` and `kubectl` installed
4. Network access to the cluster

### Optional
- MCP servers deployed for MCP routing and end-to-end tests
- `jq` for prettier JSON output (not required)

## Running from Outside the Cluster

If running tests from outside the cluster, use port-forwarding:

```bash
# Terminal 1: Port-forward gateway
kubectl port-forward -n agensys-codereview-demo svc/agentgateway 9080:9080 9081:9081 9082:9082 9083:9083 15020:15020 15021:15021

# Terminal 2: Set gateway host to localhost
export GATEWAY_HOST="localhost"

# Terminal 2: Run tests
./run-all-tests.sh
```

## Test Configuration

Each test script uses environment variables that can be customized:

```bash
# Override gateway hostname
export GATEWAY_HOST="agentgateway.agensys-codereview-demo.svc.cluster.local"

# Override agent ID
export AGENT_ID="pr-reviewer"

# Run specific test
./test-security-policies.sh
```

## Understanding Test Results

### Success Output
```
=== Test Results ===
Passed: 6
Failed: 0

✓ All security policy tests passed!
```

### Failure Output
```
=== Test Results ===
Passed: 4
Failed: 2

✗ Some tests failed
```

Check the detailed output above the summary to see which specific tests failed and why.

## Common Issues

### Tests Fail with "Connection Refused"
**Cause:** Gateway not accessible  
**Fix:**
```bash
kubectl get svc agentgateway -n agensys-codereview-demo
kubectl get pods -n agensys-codereview-demo
```

### Tests Fail with HTTP 401/403
**Cause:** API keys not configured or invalid  
**Fix:**
```bash
# Verify secrets exist
kubectl get secret api-secrets -n agensys-codereview-demo

# Check if keys are loaded
kubectl exec -n agensys-codereview-demo deployment/agentgateway -- env | grep API_KEY
```

### MCP Tests Skip or Fail
**Cause:** MCP servers not deployed  
**Fix:**
```bash
kubectl apply -f ../06-mcp-servers.yaml
kubectl wait --for=condition=ready pod -l app=github-mcp -n agensys-codereview-demo
```

### Rate Limit Tests Inconsistent
**Cause:** Previous test runs consuming rate limit quota  
**Fix:** Wait 60 seconds between test runs for rate limits to reset

## Adding Custom Tests

Create a new test script following this template:

```bash
#!/bin/bash
set -e

GATEWAY_HOST="${GATEWAY_HOST:-agentgateway.agensys-codereview-demo.svc.cluster.local}"
PASS=0
FAIL=0

echo "=== My Custom Test ==="

# Test 1
echo "Test 1: Description..."
RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null http://${GATEWAY_HOST}:9080/test)
if [ "$RESPONSE" = "200" ]; then
  echo "✓ Test passed"
  ((PASS++))
else
  echo "✗ Test failed"
  ((FAIL++))
fi

# Summary
echo "Passed: $PASS / Failed: $FAIL"
[ $FAIL -eq 0 ] && exit 0 || exit 1
```

Make it executable and add to `run-all-tests.sh`:
```bash
chmod +x test-custom.sh
```

## CI/CD Integration

Run tests in CI pipelines:

```yaml
# GitHub Actions example
- name: Run AgentGateway Tests
  run: |
    cd test-scripts
    ./run-all-tests.sh
```

## Debugging Failed Tests

Enable verbose output:
```bash
bash -x ./test-security-policies.sh
```

Check gateway logs during test execution:
```bash
kubectl logs -f -n agensys-codereview-demo -l app.kubernetes.io/component=agentgateway
```

View test request/response details:
```bash
# Tests save responses to /tmp/
cat /tmp/anthropic_response.json | jq .
cat /tmp/metrics.txt
```

## Contributing

When adding new tests:
1. Follow the existing test script structure
2. Use consistent pass/fail counting
3. Include clear test descriptions
4. Add test to `run-all-tests.sh`
5. Update this README with test details
