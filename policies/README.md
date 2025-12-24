# Zero-Trust Network Policies for Autonomous Code Review System

This directory contains Istio Ambient Mode authorization policies that implement a comprehensive Zero-Trust Network Architecture (ZTNA) for our AI-powered code review system.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Policy Files](#policy-files)
- [Quick Start](#quick-start)
- [Policy Details](#policy-details)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)

## Overview

These policies implement a **default-deny security model** where:
- All traffic is blocked by default
- Only explicitly allowed communication paths are permitted
- Each component has the minimum necessary access (least privilege)
- Policies use SPIFFE identities for cryptographic authentication
- Network-level enforcement prevents policy bypass

### Security Objectives

✅ **Agents can only access their designated LLMs**
- PR Agent → OpenAI GPT-4 only
- Summary Agent → Anthropic Claude only

✅ **Agents can only access authorized MCP servers**
- All coordination flows through Orchestrator
- No direct agent-to-agent communication

✅ **MCP servers can only access their designated tools**
- MCP Scanning → Semgrep Cloud only
- GitHub MCP → GitHub API only

✅ **Default deny everywhere**
- Positive security model
- Zero implicit trust

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      GitHub Repository                       │
│                            ↓ webhook                         │
└──────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster (agensys-demo-1)           │
│                                                              │
│  ┌──────────────────┐                                       │
│  │  Orchestrator    │─────┐                                 │
│  │     Agent        │     │                                 │
│  └──────────────────┘     │                                 │
│           │                │                                 │
│           │ (mTLS)         │ (mTLS)                         │
│           ↓                ↓                                 │
│  ┌──────────────┐   ┌──────────────┐                       │
│  │  PR Agent    │   │ MCP Scanning │                       │
│  │              │   │   Server     │                       │
│  └──────────────┘   └──────────────┘                       │
│         │                   │                                │
│         │ (HTTPS)           │ (HTTPS)                       │
│         ↓                   ↓                                │
│  ┌──────────────┐   ┌──────────────┐                       │
│  │  OpenAI API  │   │ Semgrep Cloud│ (External Services)   │
│  └──────────────┘   └──────────────┘                       │
└─────────────────────────────────────────────────────────────┘
```

### Communication Matrix

| Source | Destination | Allowed | Protocol | Policy File |
|--------|-------------|---------|----------|-------------|
| Orchestrator | PR Agent | ✅ | mTLS | 02-orchestrator-policies.yaml |
| Orchestrator | MCP Scanning | ✅ | mTLS | 02-orchestrator-policies.yaml |
| Orchestrator | Summary Agent | ✅ | mTLS | 02-orchestrator-policies.yaml |
| Orchestrator | GitHub MCP | ✅ | mTLS | 02-orchestrator-policies.yaml |
| PR Agent | OpenAI API | ✅ | HTTPS | 03-pr-agent-policies.yaml |
| Summary Agent | Anthropic API | ✅ | HTTPS | 04-summary-agent-policies.yaml |
| MCP Scanning | Semgrep Cloud | ✅ | HTTPS | 05-mcp-scanning-policies.yaml |
| GitHub MCP | GitHub API | ✅ | HTTPS | 06-mcp-github-policies.yaml |
| PR Agent | Summary Agent | ❌ | - | (blocked by default-deny) |
| PR Agent | Anthropic API | ❌ | - | (blocked by default-deny) |
| Any | Any (unlisted) | ❌ | - | 01-default-deny.yaml |

## Policy Files

### Core Policies

1. **`01-default-deny.yaml`** - Global default deny for all traffic
2. **`02-orchestrator-policies.yaml`** - Orchestrator to agents/MCP servers
3. **`03-pr-agent-policies.yaml`** - PR Agent to OpenAI API
4. **`04-summary-agent-policies.yaml`** - Summary Agent to Anthropic API
5. **`05-mcp-scanning-policies.yaml`** - MCP Scanning to Semgrep
6. **`06-mcp-github-policies.yaml`** - GitHub MCP to GitHub API

## Quick Start

### Prerequisites

- Kubernetes cluster with Istio Ambient Mode installed
- Namespace `agensys-demo-1` created and labeled for ambient mode:
  ```bash
  kubectl create namespace agensys-demo-1
  kubectl label namespace agensys-demo-1 istio.io/dataplane-mode=ambient
  ```
- All agents and MCP servers deployed in `agensys-demo-1` namespace

### Apply Policies

**Option 1: Apply directly from GitHub**
```bash
# Apply in order (default-deny MUST be first!)
kubectl apply -f https://raw.githubusercontent.com/Mirantis/blog-material/main/autonomous-code-review/policies/01-default-deny.yaml
kubectl apply -f https://raw.githubusercontent.com/Mirantis/blog-material/main/autonomous-code-review/policies/02-orchestrator-policies.yaml
kubectl apply -f https://raw.githubusercontent.com/Mirantis/blog-material/main/autonomous-code-review/policies/03-pr-agent-policies.yaml
kubectl apply -f https://raw.githubusercontent.com/Mirantis/blog-material/main/autonomous-code-review/policies/04-summary-agent-policies.yaml
kubectl apply -f https://raw.githubusercontent.com/Mirantis/blog-material/main/autonomous-code-review/policies/05-mcp-scanning-policies.yaml
kubectl apply -f https://raw.githubusercontent.com/Mirantis/blog-material/main/autonomous-code-review/policies/06-mcp-github-policies.yaml
```

**Option 2: Clone and apply locally**
```bash
git clone https://github.com/Mirantis/blog-material.git
cd blog-material/autonomous-code-review/policies
kubectl apply -f 01-default-deny.yaml
kubectl apply -f 02-orchestrator-policies.yaml
kubectl apply -f 03-pr-agent-policies.yaml
kubectl apply -f 04-summary-agent-policies.yaml
kubectl apply -f 05-mcp-scanning-policies.yaml
kubectl apply -f 06-mcp-github-policies.yaml
```

### Verify Installation

```bash
# Check all policies are created
kubectl get authorizationpolicies -n agensys-demo-1

# Check ServiceEntries for external APIs
kubectl get serviceentries -n agensys-demo-1

# Quick validation of entire setup
chmod +x quick-check.sh
./quick-check.sh
```

Expected output from quick-check.sh:
```
✅ Found 9 authorization policies
✅ Found 4 service entries
✅ Namespace is in ambient mode
✅ 3 ztunnel pod(s) running
✅ Found 5 pod(s)
✅ All checks passed!
```

For detailed policy listing:
```
NAME                                  AGE
default-deny-all                      1m
allow-orchestrator-to-pr-agent        1m
allow-orchestrator-to-mcp-scanning    1m
allow-orchestrator-to-summary-agent   1m
allow-orchestrator-to-github-mcp      1m
allow-pr-agent-to-openai              1m
allow-summary-agent-to-anthropic      1m
allow-mcp-scanning-to-semgrep         1m
allow-github-mcp-to-github            1m
```

## Policy Details

### 1. Default Deny Policy (`01-default-deny.yaml`)

**Purpose**: Establishes a global deny-all baseline. Any traffic not explicitly allowed will be blocked.

**Key Configuration**:
- Applies to all workloads in namespace (empty selector)
- No rules specified = deny everything
- Enforced by ztunnel at destination

**SPIFFE Identity**: N/A (applies to all)

**Why it matters**: Inverts the security model from "allow by default" to "deny by default", requiring explicit permits for all communication.

---

### 2. Orchestrator Policies (`02-orchestrator-policies.yaml`)

**Purpose**: Allows the Orchestrator to coordinate workflow by communicating with all agents and MCP servers.

**Allowed Connections**:
- Orchestrator → PR Agent (port 8080, POST)
- Orchestrator → MCP Scanning (port 3000)
- Orchestrator → Summary Agent (port 8080, POST)
- Orchestrator → GitHub MCP (port 3000)

**SPIFFE Identity**: `cluster.local/ns/agensys-demo-1/sa/orchestrator-agent`

**Why it matters**: Enforces that all workflow coordination flows through the Orchestrator. Agents cannot communicate directly with each other.

---

### 3. PR Agent Policy (`03-pr-agent-policies.yaml`)

**Purpose**: Permits PR Agent to access OpenAI API for code analysis via GPT-4.

**Allowed Connections**:
- PR Agent → api.openai.com:443 (HTTPS)

**SPIFFE Identity**: `cluster.local/ns/agensys-demo-1/sa/pr-agent`

**Components**:
- `ServiceEntry`: Makes OpenAI API visible to mesh
- `AuthorizationPolicy`: Restricts access to PR Agent only

**Why it matters**: 
- Prevents PR Agent from accessing other LLMs (cost control)
- Blocks unauthorized external connections
- Enforces architectural decision (PR Agent = OpenAI only)

---

### 4. Summary Agent Policy (`04-summary-agent-policies.yaml`)

**Purpose**: Permits Summary Agent to access Anthropic API for executive summaries via Claude.

**Allowed Connections**:
- Summary Agent → api.anthropic.com:443 (HTTPS)

**SPIFFE Identity**: `cluster.local/ns/agensys-demo-1/sa/summary-agent`

**Components**:
- `ServiceEntry`: Makes Anthropic API visible to mesh
- `AuthorizationPolicy`: Restricts access to Summary Agent only

**Why it matters**:
- Separates LLM usage by function (GPT for code, Claude for summaries)
- Prevents cross-contamination of LLM providers
- Enforces architectural separation

---

### 5. MCP Scanning Policy (`05-mcp-scanning-policies.yaml`)

**Purpose**: Permits MCP Scanning Server to access Semgrep Cloud for vulnerability detection.

**Allowed Connections**:
- MCP Scanning → semgrep.dev:443 (HTTPS)

**SPIFFE Identity**: `cluster.local/ns/agensys-demo-1/sa/mcp-code-scanning`

**Components**:
- `ServiceEntry`: Makes Semgrep Cloud visible to mesh
- `AuthorizationPolicy`: Restricts access to MCP Scanning only

**Why it matters**:
- MCP server can only access its designated tool
- Limits blast radius if MCP server is compromised
- Clear audit trail for all Semgrep access

---

### 6. GitHub MCP Policy (`06-mcp-github-policies.yaml`)

**Purpose**: Permits GitHub MCP Server to access GitHub API for posting PR comments.

**Allowed Connections**:
- GitHub MCP → api.github.com:443 (HTTPS)

**SPIFFE Identity**: `cluster.local/ns/agensys-demo-1/sa/github-mcp-server`

**Components**:
- `ServiceEntry`: Makes GitHub API visible to mesh
- `AuthorizationPolicy`: Restricts access to GitHub MCP only

**Why it matters**:
- Single point of GitHub integration (centralized, auditable)
- Agents cannot bypass workflow to post directly
- All GitHub interactions flow through one component

---

## Testing

### Comprehensive Test Script

Run the complete validation suite:

```bash
#!/bin/bash
# connectivity-test.sh

echo "=== Testing Orchestrator Connectivity ==="
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- curl -s -m 5 http://pr-agent:8080/health && echo "✅ Orchestrator → PR Agent: PASS" || echo "❌ Orchestrator → PR Agent: FAIL"
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- curl -s -m 5 http://mcp-code-scanning:3000/health && echo "✅ Orchestrator → MCP Scanning: PASS" || echo "❌ Orchestrator → MCP Scanning: FAIL"
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- curl -s -m 5 http://executive-summary-agent:8080/health && echo "✅ Orchestrator → Summary: PASS" || echo "❌ Orchestrator → Summary: FAIL"
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- curl -s -m 5 http://github-mcp-server:3000/health && echo "✅ Orchestrator → GitHub MCP: PASS" || echo "❌ Orchestrator → GitHub MCP: FAIL"

echo -e "\n=== Testing Agent to External LLM Connectivity ==="
kubectl exec -n agensys-demo-1 deploy/pr-agent -- curl -s -m 5 https://api.openai.com/v1/models > /dev/null 2>&1 && echo "✅ PR Agent → OpenAI: PASS" || echo "❌ PR Agent → OpenAI: FAIL"
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- curl -s -m 5 https://api.anthropic.com/v1/messages > /dev/null 2>&1 && echo "✅ Summary Agent → Anthropic: PASS" || echo "❌ Summary Agent → Anthropic: FAIL"

echo -e "\n=== Testing MCP Server to External Tool Connectivity ==="
kubectl exec -n agensys-demo-1 deploy/mcp-code-scanning -- curl -s -m 5 https://semgrep.dev > /dev/null 2>&1 && echo "✅ MCP Scanning → Semgrep: PASS" || echo "❌ MCP Scanning → Semgrep: FAIL"
kubectl exec -n agensys-demo-1 deploy/github-mcp-server -- curl -s -m 5 https://api.github.com > /dev/null 2>&1 && echo "✅ GitHub MCP → GitHub: PASS" || echo "❌ GitHub MCP → GitHub: FAIL"

echo -e "\n=== Testing DENIED Inter-Agent Communication (should fail) ==="
kubectl exec -n agensys-demo-1 deploy/pr-agent -- curl -s -m 5 http://executive-summary-agent:8080/health > /dev/null 2>&1 && echo "❌ PR Agent → Summary Agent: UNEXPECTED PASS" || echo "✅ PR Agent → Summary Agent: CORRECTLY BLOCKED"
kubectl exec -n agensys-demo-1 deploy/pr-agent -- curl -s -m 5 http://github-mcp-server:3000/health > /dev/null 2>&1 && echo "❌ PR Agent → GitHub MCP: UNEXPECTED PASS" || echo "✅ PR Agent → GitHub MCP: CORRECTLY BLOCKED"

echo -e "\n=== Testing DENIED Cross-LLM Access (should fail) ==="
kubectl exec -n agensys-demo-1 deploy/pr-agent -- curl -s -m 5 https://api.anthropic.com > /dev/null 2>&1 && echo "❌ PR Agent → Anthropic: UNEXPECTED PASS" || echo "✅ PR Agent → Anthropic: CORRECTLY BLOCKED"
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- curl -s -m 5 https://api.openai.com > /dev/null 2>&1 && echo "❌ Summary Agent → OpenAI: UNEXPECTED PASS" || echo "✅ Summary Agent → OpenAI: CORRECTLY BLOCKED"

echo -e "\n=== Testing DENIED MCP Server Cross-Tool Access (should fail) ==="
kubectl exec -n agensys-demo-1 deploy/mcp-code-scanning -- curl -s -m 5 https://api.github.com > /dev/null 2>&1 && echo "❌ MCP Scanning → GitHub: UNEXPECTED PASS" || echo "✅ MCP Scanning → GitHub: CORRECTLY BLOCKED"
kubectl exec -n agensys-demo-1 deploy/github-mcp-server -- curl -s -m 5 https://semgrep.dev > /dev/null 2>&1 && echo "❌ GitHub MCP → Semgrep: UNEXPECTED PASS" || echo "✅ GitHub MCP → Semgrep: CORRECTLY BLOCKED"

echo -e "\n=== Policy Validation Complete ==="
```

Save as `connectivity-test.sh` and run:
```bash
chmod +x connectivity-test.sh
./connectivity-test.sh
```

### Individual Policy Tests

#### Test 1: Default Deny
```bash
# Should FAIL (timeout) - all traffic blocked by default
kubectl exec -n agensys-demo-1 deploy/pr-agent -- curl -v -m 5 http://executive-summary-agent:8080/health
```

Expected: `curl: (28) Connection timed out`

#### Test 2: Orchestrator Access
```bash
# Should SUCCEED - allowed by policy
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- curl -v -m 5 http://pr-agent:8080/health
```

Expected: `HTTP/1.1 200 OK`

#### Test 3: PR Agent to OpenAI
```bash
# Should SUCCEED (gets 401 but connection allowed)
kubectl exec -n agensys-demo-1 deploy/pr-agent -- curl -v -m 10 https://api.openai.com/v1/models
```

Expected: `HTTP/1.1 401 Unauthorized` (connection succeeded, auth failed as expected)

#### Test 4: PR Agent to Anthropic (blocked)
```bash
# Should FAIL - PR Agent cannot access Anthropic
kubectl exec -n agensys-demo-1 deploy/pr-agent -- curl -v -m 10 https://api.anthropic.com
```

Expected: `curl: (28) Connection timed out`

#### Test 5: Summary Agent to Anthropic
```bash
# Should SUCCEED (gets 400 but connection allowed)
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- curl -v -m 10 https://api.anthropic.com/v1/messages
```

Expected: `HTTP/1.1 400 Bad Request` (connection succeeded, missing API key)

#### Test 6: Summary Agent to OpenAI (blocked)
```bash
# Should FAIL - Summary Agent cannot access OpenAI
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- curl -v -m 10 https://api.openai.com
```

Expected: `curl: (28) Connection timed out`

### Monitor Policy Enforcement

Watch ztunnel logs in real-time:
```bash
kubectl logs -n istio-system -l app=ztunnel -f | grep -E "(ALLOW|DENY|RBAC)"
```

Check for denied connections:
```bash
kubectl logs -n istio-system -l app=ztunnel | grep RBAC_ACCESS_DENIED
```

### Metrics

Query Prometheus for policy metrics:
```bash
kubectl exec -n istio-system deploy/istiod -- curl localhost:15014/metrics | grep istio_requests_total
```

## Troubleshooting

### Listing and Inspecting Policies

#### List All Authorization Policies

```bash
# List all AuthorizationPolicies in a specific namespace
kubectl get authorizationpolicies -n agensys-demo-1

# Short form
kubectl get authzpolicy -n agensys-demo-1

# All namespaces
kubectl get authorizationpolicies --all-namespaces

# Wide output with more details
kubectl get authorizationpolicies -n agensys-demo-1 -o wide
```

**Example output:**
```
NAME                                  AGE
default-deny-all                      5m
allow-orchestrator-to-pr-agent        5m
allow-pr-agent-to-openai              5m
```

#### List ServiceEntries (External APIs)

```bash
# List ServiceEntries (used for external API access)
kubectl get serviceentries -n agensys-demo-1

# Short form
kubectl get se -n agensys-demo-1

# All namespaces
kubectl get serviceentries --all-namespaces
```

**Example output:**
```
NAME            HOSTS                  LOCATION        RESOLUTION   AGE
openai-api      ["api.openai.com"]     MESH_EXTERNAL   DNS          5m
anthropic-api   ["api.anthropic.com"]  MESH_EXTERNAL   DNS          5m
```

#### Get Detailed Policy Information

```bash
# Get detailed YAML of a specific policy
kubectl get authorizationpolicy default-deny-all -n agensys-demo-1 -o yaml

# Get policy with specific fields
kubectl get authorizationpolicy -n agensys-demo-1 -o custom-columns=NAME:.metadata.name,ACTION:.spec.action,AGE:.metadata.creationTimestamp

# Describe a policy to see events and details
kubectl describe authorizationpolicy default-deny-all -n agensys-demo-1
```

#### List All Istio Security Resources

```bash
# List all Istio security-related resources at once
kubectl get authorizationpolicies,peerauthentications,requestauthentications -n agensys-demo-1

# Get all security policies across all namespaces
kubectl get authorizationpolicies --all-namespaces -o wide
```

#### Using istioctl (Istio CLI)

```bash
# Analyze authorization policies for issues
istioctl analyze -n agensys-demo-1

# Get policy details for a specific workload
istioctl x authz check <pod-name> -n agensys-demo-1

# Show which policies affect a pod
istioctl x describe pod <pod-name> -n agensys-demo-1

# Check Istio configuration status
istioctl proxy-status
```

#### Check Ambient Mode Configuration

```bash
# Verify namespace is in ambient mode
kubectl get namespace agensys-demo-1 -o jsonpath='{.metadata.labels.istio\.io/dataplane-mode}'
# Should output: ambient

# Check all namespaces with ambient mode enabled
kubectl get namespaces -l istio.io/dataplane-mode=ambient

# List all pods with ambient data plane annotation
kubectl get pods -n agensys-demo-1 -o custom-columns=NAME:.metadata.name,DATAPLANE:.metadata.labels.istio\.io/dataplane-mode
```

#### Check Ztunnel (Ambient Data Plane)

```bash
# List ztunnel pods
kubectl get pods -n istio-system -l app=ztunnel

# Check ztunnel logs for policy enforcement
kubectl logs -n istio-system -l app=ztunnel | grep -E "(RBAC|authz|policy)"

# See denied connections (policy violations)
kubectl logs -n istio-system -l app=ztunnel | grep RBAC_ACCESS_DENIED

# Follow ztunnel logs in real-time
kubectl logs -n istio-system -l app=ztunnel -f | grep -E "(ALLOW|DENY|RBAC)"

# Check ztunnel logs for a specific workload
kubectl logs -n istio-system -l app=ztunnel | grep "pr-agent"
```

#### Export Policies

```bash
# Export all policies to a single YAML file
kubectl get authorizationpolicies -n agensys-demo-1 -o yaml > all-policies.yaml

# Export each policy separately
for policy in $(kubectl get authorizationpolicies -n agensys-demo-1 -o name); do
    kubectl get $policy -n agensys-demo-1 -o yaml > ${policy##*/}.yaml
done

# Export ServiceEntries
kubectl get serviceentries -n agensys-demo-1 -o yaml > all-serviceentries.yaml
```

#### Monitor Policy Metrics

```bash
# Query policy metrics from istiod
kubectl exec -n istio-system deploy/istiod -- curl localhost:15014/metrics | grep istio_authorization

# Check policy cache stats
kubectl exec -n istio-system deploy/istiod -- curl localhost:15014/debug/authorizationz

# Query ztunnel metrics
kubectl exec -n istio-system -l app=ztunnel -- curl localhost:15020/stats/prometheus | grep istio
```

#### Quick Validation Script

Create a script to quickly check your ambient setup:

```bash
#!/bin/bash
# quick-check.sh

NAMESPACE="agensys-demo-1"

echo "=== Authorization Policies ==="
kubectl get authorizationpolicies -n $NAMESPACE

echo -e "\n=== Service Entries ==="
kubectl get serviceentries -n $NAMESPACE

echo -e "\n=== Namespace Ambient Mode ==="
kubectl get namespace $NAMESPACE -o jsonpath='{.metadata.labels.istio\.io/dataplane-mode}'
echo ""

echo -e "\n=== Ztunnel Status ==="
kubectl get pods -n istio-system -l app=ztunnel

echo -e "\n=== Workloads in Namespace ==="
kubectl get pods -n $NAMESPACE

echo -e "\n=== Recent Policy Denials (Last 10) ==="
kubectl logs -n istio-system -l app=ztunnel --tail=100 | grep RBAC_ACCESS_DENIED | tail -10
```

Save as `quick-check.sh`, make executable with `chmod +x quick-check.sh`, and run.

---

## Troubleshooting

### Policy Not Taking Effect

1. **Verify policy is applied**:
   ```bash
   kubectl get authorizationpolicy <policy-name> -n agensys-demo-1 -o yaml
   ```

2. **Check ztunnel is running**:
   ```bash
   kubectl get pods -n istio-system -l app=ztunnel
   ```

3. **Verify namespace is in ambient mode**:
   ```bash
   kubectl get namespace agensys-demo-1 -o jsonpath='{.metadata.labels}'
   ```
   Should show: `"istio.io/dataplane-mode":"ambient"`

### Allowed Connection Failing

1. **Check workload labels match policy selector**:
   ```bash
   kubectl get pod -n agensys-demo-1 -l app=pr-agent --show-labels
   ```

2. **Verify SPIFFE identity**:
   ```bash
   kubectl get pod -n agensys-demo-1 <pod-name> -o jsonpath='{.spec.serviceAccountName}'
   ```

3. **Check ServiceEntry exists** (for external APIs):
   ```bash
   kubectl get serviceentry -n agensys-demo-1
   ```

### Unexpected Blocking

1. **Review ztunnel logs**:
   ```bash
   kubectl logs -n istio-system -l app=ztunnel | grep RBAC_ACCESS_DENIED
   ```

2. **Check policy order** - default-deny should be applied first

3. **Verify ports** - ensure policy allows the correct port

## Security Best Practices

1. **Always apply default-deny first** - establishes baseline security
2. **Use SPIFFE identities** - never rely on IP addresses
3. **Minimum necessary access** - grant least privilege
4. **Monitor denied connections** - set up alerts for RBAC_ACCESS_DENIED
5. **Regular policy audits** - review and update policies as architecture evolves
6. **Test before production** - validate policies in staging environment

## Contributing

When adding new policies:
1. Follow naming convention: `NN-descriptive-name.yaml`
2. Include clear comments in YAML
3. Add corresponding test to `connectivity-test.sh`
4. Update this README with policy details
5. Document SPIFFE identities used

## License

[Your License Here]

## References

- [Istio Authorization Policies](https://istio.io/latest/docs/reference/config/security/authorization-policy/)
- [Istio Ambient Mode](https://istio.io/latest/docs/ambient/)
- [SPIFFE Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md)
- [Blog Post: Part 1 - Ambient Mode Setup](https://link-to-part-1)
- [Blog Post: Part 2 - Zero-Trust Policies](https://link-to-part-2)
