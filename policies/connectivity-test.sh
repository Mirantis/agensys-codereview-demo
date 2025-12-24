#!/bin/bash
# ==============================================================================
# Zero-Trust Policy Validation Script
# File: connectivity-test.sh
# ==============================================================================
#
# PURPOSE:
#   Comprehensive test suite to validate that Zero-Trust network policies are
#   correctly enforced. Tests both allowed and denied connections.
#
# USAGE:
#   chmod +x connectivity-test.sh
#   ./connectivity-test.sh
#
# REQUIREMENTS:
#   - kubectl configured with access to cluster
#   - All agents and MCP servers deployed in agensys-demo-1 namespace
#   - All policies applied (01-06)
#
# EXIT CODES:
#   0 - All tests passed
#   1 - One or more tests failed
#
# ==============================================================================

set -e

NAMESPACE="agensys-demo-1"
TIMEOUT=5
LONG_TIMEOUT=10
PASS_COUNT=0
FAIL_COUNT=0

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=============================================================="
echo "  Zero-Trust Network Policy Validation"
echo "  Namespace: ${NAMESPACE}"
echo "=============================================================="
echo ""

# ==============================================================================
# Helper Functions
# ==============================================================================

test_pass() {
    echo -e "${GREEN}✅ $1${NC}"
    ((PASS_COUNT++))
}

test_fail() {
    echo -e "${RED}❌ $1${NC}"
    ((FAIL_COUNT++))
}

test_info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

# ==============================================================================
# Test 1: Verify Policies Exist
# ==============================================================================

echo "=== Test 1: Verifying Policies Are Applied ==="
echo ""

EXPECTED_POLICIES=(
    "default-deny-all"
    "allow-orchestrator-to-pr-agent"
    "allow-orchestrator-to-mcp-scanning"
    "allow-orchestrator-to-summary-agent"
    "allow-orchestrator-to-github-mcp"
    "allow-pr-agent-to-openai"
    "allow-summary-agent-to-anthropic"
    "allow-mcp-scanning-to-semgrep"
    "allow-github-mcp-to-github"
)

for policy in "${EXPECTED_POLICIES[@]}"; do
    if kubectl get authorizationpolicy "$policy" -n "$NAMESPACE" &>/dev/null; then
        test_pass "Policy exists: $policy"
    else
        test_fail "Policy missing: $policy"
    fi
done

echo ""

# ==============================================================================
# Test 2: Orchestrator Connectivity (Should PASS)
# ==============================================================================

echo "=== Test 2: Testing Orchestrator Connectivity (Allowed) ==="
echo ""

# Test 2.1: Orchestrator → PR Agent
if kubectl exec -n "$NAMESPACE" deploy/orchestrator-agent -- \
    curl -s -m "$TIMEOUT" http://pr-agent:8080/health > /dev/null 2>&1; then
    test_pass "Orchestrator → PR Agent"
else
    test_fail "Orchestrator → PR Agent (should be allowed)"
fi

# Test 2.2: Orchestrator → MCP Scanning
if kubectl exec -n "$NAMESPACE" deploy/orchestrator-agent -- \
    curl -s -m "$TIMEOUT" http://mcp-code-scanning:3000/health > /dev/null 2>&1; then
    test_pass "Orchestrator → MCP Scanning"
else
    test_fail "Orchestrator → MCP Scanning (should be allowed)"
fi

# Test 2.3: Orchestrator → Summary Agent
if kubectl exec -n "$NAMESPACE" deploy/orchestrator-agent -- \
    curl -s -m "$TIMEOUT" http://executive-summary-agent:8080/health > /dev/null 2>&1; then
    test_pass "Orchestrator → Summary Agent"
else
    test_fail "Orchestrator → Summary Agent (should be allowed)"
fi

# Test 2.4: Orchestrator → GitHub MCP
if kubectl exec -n "$NAMESPACE" deploy/orchestrator-agent -- \
    curl -s -m "$TIMEOUT" http://github-mcp-server:3000/health > /dev/null 2>&1; then
    test_pass "Orchestrator → GitHub MCP"
else
    test_fail "Orchestrator → GitHub MCP (should be allowed)"
fi

echo ""

# ==============================================================================
# Test 3: Agent to External LLM Connectivity (Should PASS)
# ==============================================================================

echo "=== Test 3: Testing Agent to External LLM Connectivity (Allowed) ==="
echo ""

# Test 3.1: PR Agent → OpenAI
if kubectl exec -n "$NAMESPACE" deploy/pr-agent -- \
    curl -s -m "$LONG_TIMEOUT" https://api.openai.com/v1/models > /dev/null 2>&1; then
    test_pass "PR Agent → OpenAI API"
else
    test_fail "PR Agent → OpenAI API (should be allowed)"
fi

# Test 3.2: Summary Agent → Anthropic
if kubectl exec -n "$NAMESPACE" deploy/executive-summary-agent -- \
    curl -s -m "$LONG_TIMEOUT" https://api.anthropic.com/v1/messages > /dev/null 2>&1; then
    test_pass "Summary Agent → Anthropic API"
else
    test_fail "Summary Agent → Anthropic API (should be allowed)"
fi

echo ""

# ==============================================================================
# Test 4: MCP Server to External Tool Connectivity (Should PASS)
# ==============================================================================

echo "=== Test 4: Testing MCP Server to External Tool Connectivity (Allowed) ==="
echo ""

# Test 4.1: MCP Scanning → Semgrep
if kubectl exec -n "$NAMESPACE" deploy/mcp-code-scanning -- \
    curl -s -m "$LONG_TIMEOUT" https://semgrep.dev > /dev/null 2>&1; then
    test_pass "MCP Scanning → Semgrep Cloud"
else
    test_fail "MCP Scanning → Semgrep Cloud (should be allowed)"
fi

# Test 4.2: GitHub MCP → GitHub API
if kubectl exec -n "$NAMESPACE" deploy/github-mcp-server -- \
    curl -s -m "$LONG_TIMEOUT" https://api.github.com > /dev/null 2>&1; then
    test_pass "GitHub MCP → GitHub API"
else
    test_fail "GitHub MCP → GitHub API (should be allowed)"
fi

echo ""

# ==============================================================================
# Test 5: Inter-Agent Communication (Should FAIL - Blocked)
# ==============================================================================

echo "=== Test 5: Testing Denied Inter-Agent Communication (Should Block) ==="
echo ""

# Test 5.1: PR Agent → Summary Agent (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/pr-agent -- \
    curl -s -m "$TIMEOUT" http://executive-summary-agent:8080/health > /dev/null 2>&1; then
    test_fail "PR Agent → Summary Agent (should be BLOCKED)"
else
    test_pass "PR Agent → Summary Agent (correctly blocked)"
fi

# Test 5.2: PR Agent → GitHub MCP (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/pr-agent -- \
    curl -s -m "$TIMEOUT" http://github-mcp-server:3000/health > /dev/null 2>&1; then
    test_fail "PR Agent → GitHub MCP (should be BLOCKED)"
else
    test_pass "PR Agent → GitHub MCP (correctly blocked)"
fi

# Test 5.3: Summary Agent → PR Agent (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/executive-summary-agent -- \
    curl -s -m "$TIMEOUT" http://pr-agent:8080/health > /dev/null 2>&1; then
    test_fail "Summary Agent → PR Agent (should be BLOCKED)"
else
    test_pass "Summary Agent → PR Agent (correctly blocked)"
fi

echo ""

# ==============================================================================
# Test 6: Cross-LLM Access (Should FAIL - Blocked)
# ==============================================================================

echo "=== Test 6: Testing Denied Cross-LLM Access (Should Block) ==="
echo ""

# Test 6.1: PR Agent → Anthropic (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/pr-agent -- \
    curl -s -m "$TIMEOUT" https://api.anthropic.com > /dev/null 2>&1; then
    test_fail "PR Agent → Anthropic (should be BLOCKED)"
else
    test_pass "PR Agent → Anthropic (correctly blocked)"
fi

# Test 6.2: Summary Agent → OpenAI (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/executive-summary-agent -- \
    curl -s -m "$TIMEOUT" https://api.openai.com > /dev/null 2>&1; then
    test_fail "Summary Agent → OpenAI (should be BLOCKED)"
else
    test_pass "Summary Agent → OpenAI (correctly blocked)"
fi

echo ""

# ==============================================================================
# Test 7: MCP Server Cross-Tool Access (Should FAIL - Blocked)
# ==============================================================================

echo "=== Test 7: Testing Denied MCP Server Cross-Tool Access (Should Block) ==="
echo ""

# Test 7.1: MCP Scanning → GitHub (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/mcp-code-scanning -- \
    curl -s -m "$TIMEOUT" https://api.github.com > /dev/null 2>&1; then
    test_fail "MCP Scanning → GitHub (should be BLOCKED)"
else
    test_pass "MCP Scanning → GitHub (correctly blocked)"
fi

# Test 7.2: GitHub MCP → Semgrep (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/github-mcp-server -- \
    curl -s -m "$TIMEOUT" https://semgrep.dev > /dev/null 2>&1; then
    test_fail "GitHub MCP → Semgrep (should be BLOCKED)"
else
    test_pass "GitHub MCP → Semgrep (correctly blocked)"
fi

# Test 7.3: MCP Scanning → OpenAI (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/mcp-code-scanning -- \
    curl -s -m "$TIMEOUT" https://api.openai.com > /dev/null 2>&1; then
    test_fail "MCP Scanning → OpenAI (should be BLOCKED)"
else
    test_pass "MCP Scanning → OpenAI (correctly blocked)"
fi

# Test 7.4: GitHub MCP → Anthropic (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/github-mcp-server -- \
    curl -s -m "$TIMEOUT" https://api.anthropic.com > /dev/null 2>&1; then
    test_fail "GitHub MCP → Anthropic (should be BLOCKED)"
else
    test_pass "GitHub MCP → Anthropic (correctly blocked)"
fi

echo ""

# ==============================================================================
# Test 8: Agents Cannot Access Arbitrary External Sites (Should FAIL - Blocked)
# ==============================================================================

echo "=== Test 8: Testing Denied Arbitrary External Access (Should Block) ==="
echo ""

# Test 8.1: PR Agent → Google (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/pr-agent -- \
    curl -s -m "$TIMEOUT" https://google.com > /dev/null 2>&1; then
    test_fail "PR Agent → Google (should be BLOCKED)"
else
    test_pass "PR Agent → Google (correctly blocked)"
fi

# Test 8.2: Summary Agent → Example.com (should be blocked)
if kubectl exec -n "$NAMESPACE" deploy/executive-summary-agent -- \
    curl -s -m "$TIMEOUT" https://example.com > /dev/null 2>&1; then
    test_fail "Summary Agent → Example.com (should be BLOCKED)"
else
    test_pass "Summary Agent → Example.com (correctly blocked)"
fi

echo ""

# ==============================================================================
# Summary
# ==============================================================================

echo "=============================================================="
echo "  Test Summary"
echo "=============================================================="
echo ""
echo "Passed: ${PASS_COUNT}"
echo "Failed: ${FAIL_COUNT}"
echo ""

if [ "$FAIL_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed! Zero-Trust policies are correctly enforced.${NC}"
    exit 0
else
    echo -e "${RED}❌ ${FAIL_COUNT} test(s) failed. Please review policy configuration.${NC}"
    echo ""
    echo "Troubleshooting tips:"
    echo "1. Verify all policies are applied: kubectl get authorizationpolicies -n ${NAMESPACE}"
    echo "2. Check ztunnel logs: kubectl logs -n istio-system -l app=ztunnel | grep RBAC"
    echo "3. Verify namespace is in ambient mode: kubectl get ns ${NAMESPACE} -o yaml | grep dataplane-mode"
    echo "4. Ensure all workloads are deployed: kubectl get pods -n ${NAMESPACE}"
    exit 1
fi
