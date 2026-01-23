#!/bin/bash
# ============================================================================
# Run All AgentGateway Tests
# ============================================================================
# Runs all test suites and provides summary
# ============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOTAL_PASS=0
TOTAL_FAIL=0

echo "════════════════════════════════════════════════════════════════"
echo "  AgentGateway Test Suite"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Check if gateway is accessible
echo "Checking gateway accessibility..."
if ! kubectl get svc agentgateway -n agensys-codereview-demo &> /dev/null; then
  echo "✗ AgentGateway service not found!"
  echo "  Make sure you've deployed the gateway first:"
  echo "  kubectl apply -f manifests/"
  exit 1
fi
echo "✓ Gateway service found"
echo ""

# Make scripts executable
chmod +x "${SCRIPT_DIR}"/*.sh

# Test 1: Security Policies
echo "════════════════════════════════════════════════════════════════"
echo "Running Security Policy Tests..."
echo "════════════════════════════════════════════════════════════════"
if bash "${SCRIPT_DIR}/test-security-policies.sh"; then
  ((TOTAL_PASS++))
else
  ((TOTAL_FAIL++))
fi
echo ""

# Test 2: LLM Routing
echo "════════════════════════════════════════════════════════════════"
echo "Running LLM Routing Tests..."
echo "════════════════════════════════════════════════════════════════"
if bash "${SCRIPT_DIR}/test-llm-routing.sh"; then
  ((TOTAL_PASS++))
else
  ((TOTAL_FAIL++))
fi
echo ""

# Test 3: MCP Routing (optional - only if MCP servers deployed)
if kubectl get deployment github-mcp -n agensys-codereview-demo &> /dev/null; then
  echo "════════════════════════════════════════════════════════════════"
  echo "Running MCP Routing Tests..."
  echo "════════════════════════════════════════════════════════════════"
  if bash "${SCRIPT_DIR}/test-mcp-routing.sh"; then
    ((TOTAL_PASS++))
  else
    ((TOTAL_FAIL++))
  fi
  echo ""
else
  echo "⚠ MCP servers not deployed - skipping MCP routing tests"
  echo "  Deploy MCP servers with: kubectl apply -f 06-mcp-servers.yaml"
  echo ""
fi

# Test 4: Observability
echo "════════════════════════════════════════════════════════════════"
echo "Running Observability Tests..."
echo "════════════════════════════════════════════════════════════════"
if bash "${SCRIPT_DIR}/test-observability.sh"; then
  ((TOTAL_PASS++))
else
  ((TOTAL_FAIL++))
fi
echo ""

# Test 5: End-to-End Workflow (optional - requires MCP servers)
if kubectl get deployment github-mcp -n agensys-codereview-demo &> /dev/null; then
  echo "════════════════════════════════════════════════════════════════"
  echo "Running End-to-End Workflow Test..."
  echo "════════════════════════════════════════════════════════════════"
  if bash "${SCRIPT_DIR}/test-end-to-end-workflow.sh"; then
    ((TOTAL_PASS++))
  else
    ((TOTAL_FAIL++))
  fi
  echo ""
fi

# Summary
echo "════════════════════════════════════════════════════════════════"
echo "  Test Summary"
echo "════════════════════════════════════════════════════════════════"
echo "Test Suites Passed: $TOTAL_PASS"
echo "Test Suites Failed: $TOTAL_FAIL"
echo ""

if [ $TOTAL_FAIL -eq 0 ]; then
  echo "✓✓✓ All test suites passed! ✓✓✓"
  exit 0
else
  echo "✗✗✗ Some test suites failed ✗✗✗"
  exit 1
fi
