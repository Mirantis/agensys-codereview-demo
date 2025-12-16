# Technical Description: Zero-Trust Network Policies for Istio Ambient Mode

## Table of Contents

1. [Overview](#overview)
2. [Policy 1: Default Deny](#policy-1-default-deny)
3. [Policy 2: Orchestrator Access](#policy-2-orchestrator-access)
4. [Policy 3: PR Agent to OpenAI](#policy-3-pr-agent-to-openai)
5. [Policy 4: Summary Agent to Anthropic](#policy-4-summary-agent-to-anthropic)
6. [Policy 5: MCP Scanning to Semgrep](#policy-5-mcp-scanning-to-semgrep)
7. [Policy 6: GitHub MCP to GitHub API](#policy-6-github-mcp-to-github-api)
8. [Understanding Key YAML Components](#understanding-key-yaml-components)
9. [Testing Methodology](#testing-methodology)

---

## Overview

This document provides in-depth technical explanations of each Zero-Trust network policy, breaking down the YAML structure and explaining how each component contributes to the security model. All policies leverage Istio Ambient Mode's ztunnel for Layer 4 enforcement and SPIFFE identities for cryptographic workload authentication.

---

## Policy 1: Default Deny

### File: `01-default-deny.yaml`

### Purpose
Establishes a global deny-all baseline for the namespace. This is the foundation of the Zero-Trust architecture where all traffic is blocked by default.

### YAML Structure

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: default-deny-all
  namespace: agensys-demo-1
spec:
  {}
```

### YAML Component Breakdown

| Component | Value | Purpose |
|-----------|-------|---------|
| `apiVersion` | `security.istio.io/v1` | Istio security API version |
| `kind` | `AuthorizationPolicy` | Resource type for access control |
| `metadata.name` | `default-deny-all` | Unique identifier for this policy |
| `metadata.namespace` | `agensys-demo-1` | Namespace where policy applies |
| `spec` | `{}` (empty) | Empty spec = deny all traffic |

### How It Works

**Empty Spec = Global Deny**
- No `selector`: Applies to ALL workloads in the namespace
- No `action`: Defaults to DENY
- No `rules`: No exceptions, deny everything

**Enforcement Point**
- Enforced by ztunnel at the **destination node**
- Applies to **inbound traffic** to all pods
- Creates a "positive security model" (explicit allow required)

### Security Impact

✅ **Prevents:**
- Accidental connections between agents
- Lateral movement if an agent is compromised
- Data exfiltration to unauthorized endpoints
- Agents accessing MCP servers they shouldn't use

⚠️ **Important:**
- This MUST be applied BEFORE any allow policies
- Without this, mesh operates in "allow-all" mode
- All subsequent policies create exceptions to this rule

### Testing

#### Kubernetes Perspective

**Check policy exists:**
```bash
kubectl get authorizationpolicy default-deny-all -n agensys-demo-1
```

Expected output:
```
NAME               AGE
default-deny-all   5m
```

**Verify policy details:**
```bash
kubectl get authorizationpolicy default-deny-all -n agensys-demo-1 -o yaml
```

Check that `spec: {}` is empty (no rules).

**Monitor enforcement:**
```bash
kubectl logs -n istio-system -l app=ztunnel | grep RBAC_ACCESS_DENIED
```

Expected: See denied connections after this policy is applied.

#### Curl Testing

**Test 1: Agent to Agent (should FAIL)**
```bash
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -v -m 5 http://executive-summary-agent:8080/health
```

**Expected Result:**
```
* Connection timed out after 5000 milliseconds
curl: (28) Connection timed out after 5000 milliseconds
command terminated with exit code 28
```

**What's happening:**
1. PR Agent sends connection attempt to Summary Agent
2. Ztunnel on destination node intercepts the connection
3. Checks authorization policies
4. Finds only default-deny policy with no matching allow rules
5. Drops the connection (no response sent)
6. Curl times out waiting for response

**Test 2: Orchestrator to PR Agent (should FAIL)**
```bash
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- \
  curl -v -m 5 http://pr-agent:8080/health
```

**Expected Result:**
```
curl: (28) Connection timed out after 5000 milliseconds
```

This proves default-deny is working - even the Orchestrator cannot connect until we add explicit allow policies.

---

## Policy 2: Orchestrator Access

### File: `02-orchestrator-policies.yaml`

### Purpose
Grants the Orchestrator Agent permission to communicate with the four components needed for workflow coordination: PR Agent, MCP Code Scanning Server, Executive Summary Agent, and GitHub MCP Server.

### YAML Structure (4 Policies)

This file contains 4 separate `AuthorizationPolicy` resources separated by `---`.

#### Policy 2a: Orchestrator → PR Agent

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-orchestrator-to-pr-agent
  namespace: agensys-demo-1
spec:
  selector:
    matchLabels:
      app: pr-agent
  action: ALLOW
  rules:
  - from:
    - source:
        principals: ["cluster.local/ns/agensys-demo-1/sa/orchestrator-agent"]
    to:
    - operation:
        ports: ["8080"]
        methods: ["POST"]
```

### YAML Component Breakdown

| Component | Value | Purpose |
|-----------|-------|---------|
| `spec.selector.matchLabels.app` | `pr-agent` | Policy applies to pods with label `app=pr-agent` |
| `spec.action` | `ALLOW` | Create exception to default-deny |
| `rules[0].from[0].source.principals` | `["cluster.local/ns/agensys-demo-1/sa/orchestrator-agent"]` | Only allow from Orchestrator's SPIFFE ID |
| `rules[0].to[0].operation.ports` | `["8080"]` | Restrict to PR Agent's API port |
| `rules[0].to[0].operation.methods` | `["POST"]` | Only allow POST requests |

### How It Works

**Selector Mechanism**
- `selector.matchLabels`: Targets specific pods using Kubernetes labels
- Policy is **applied to the destination** (PR Agent)
- Ztunnel on PR Agent's node enforces this policy

**Principal-Based Authentication**
- Uses SPIFFE identity: `cluster.local/ns/agensys-demo-1/sa/orchestrator-agent`
- Format: `cluster.local/ns/<namespace>/sa/<service-account>`
- Cryptographically verified via mTLS certificates
- Cannot be spoofed (certificate-based, not IP-based)

**Port and Method Restrictions**
- `ports: ["8080"]`: Only this specific port is accessible
- `methods: ["POST"]`: Only POST operations allowed
- Prevents unauthorized access patterns (e.g., GET requests for data exfiltration)

**The Other 3 Policies**
- `allow-orchestrator-to-mcp-scanning`: Port 3000, no method restriction
- `allow-orchestrator-to-summary-agent`: Port 8080, POST only
- `allow-orchestrator-to-github-mcp`: Port 3000, no method restriction

### Security Impact

✅ **Enforces:**
- All workflow coordination flows through Orchestrator
- No direct agent-to-agent communication
- Specific ports and methods per service
- Cryptographic identity verification

✅ **Prevents:**
- PR Agent from calling Summary Agent directly
- Agents bypassing workflow orchestration
- Unauthorized API methods (e.g., DELETE, PUT)

### Testing

#### Kubernetes Perspective

**Check all 4 policies exist:**
```bash
kubectl get authorizationpolicies -n agensys-demo-1 | grep orchestrator
```

Expected output:
```
allow-orchestrator-to-pr-agent        5m
allow-orchestrator-to-mcp-scanning    5m
allow-orchestrator-to-summary-agent   5m
allow-orchestrator-to-github-mcp      5m
```

**Verify policy targets correct workload:**
```bash
kubectl get authorizationpolicy allow-orchestrator-to-pr-agent -n agensys-demo-1 -o jsonpath='{.spec.selector.matchLabels}'
```

Expected: `{"app":"pr-agent"}`

**Check SPIFFE principal:**
```bash
kubectl get authorizationpolicy allow-orchestrator-to-pr-agent -n agensys-demo-1 -o jsonpath='{.spec.rules[0].from[0].source.principals[0]}'
```

Expected: `cluster.local/ns/agensys-demo-1/sa/orchestrator-agent`

**Verify PR Agent pods have correct label:**
```bash
kubectl get pods -n agensys-demo-1 -l app=pr-agent
```

**Verify Orchestrator uses correct ServiceAccount:**
```bash
kubectl get pods -n agensys-demo-1 -l app=orchestrator-agent -o jsonpath='{.items[0].spec.serviceAccountName}'
```

Expected: `orchestrator-agent`

#### Curl Testing

**Test 1: Orchestrator → PR Agent (should SUCCEED)**
```bash
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- \
  curl -v -m 5 http://pr-agent:8080/health
```

**Expected Result:**
```
< HTTP/1.1 200 OK
< Content-Type: application/json
{"status":"healthy","service":"pr-agent"}
```

**What's happening:**
1. Orchestrator sends POST to PR Agent:8080
2. Ztunnel on Orchestrator's node establishes mTLS connection
3. Presents Orchestrator's SPIFFE certificate
4. Ztunnel on PR Agent's node receives connection
5. Checks authorization policies
6. Finds `allow-orchestrator-to-pr-agent` policy
7. Verifies source principal matches: ✅
8. Verifies port matches (8080): ✅
9. Allows connection through
10. PR Agent receives request and responds

**Test 2: Orchestrator → MCP Scanning (should SUCCEED)**
```bash
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- \
  curl -v -m 5 http://mcp-code-scanning:3000/health
```

**Expected Result:**
```
< HTTP/1.1 200 OK
{"status":"healthy","service":"mcp-code-scanning"}
```

**Test 3: PR Agent → Summary Agent (should FAIL - no policy allows this)**
```bash
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -v -m 5 http://executive-summary-agent:8080/health
```

**Expected Result:**
```
curl: (28) Connection timed out after 5000 milliseconds
```

**Why it fails:**
1. PR Agent attempts connection to Summary Agent
2. Ztunnel on Summary Agent's node checks policies
3. Finds policy `allow-orchestrator-to-summary-agent`
4. Checks source principal: `cluster.local/ns/agensys-demo-1/sa/pr-agent`
5. Does NOT match required principal (orchestrator-agent)
6. Falls back to default-deny
7. Connection dropped

**Test 4: Summary Agent → PR Agent (should FAIL - reverse direction)**
```bash
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- \
  curl -v -m 5 http://pr-agent:8080/health
```

**Expected Result:**
```
curl: (28) Connection timed out
```

This proves agents cannot communicate with each other, only Orchestrator can initiate connections.

---

## Policy 3: PR Agent to OpenAI

### File: `03-pr-agent-policies.yaml`

### Purpose
Allows the PR Agent to make outbound connections to the OpenAI API for GPT-4 inference. This is the ONLY external connection the PR Agent is permitted to make.

### YAML Structure (2 Resources)

This policy uses TWO Kubernetes resources:
1. `ServiceEntry`: Makes external API visible to mesh
2. `AuthorizationPolicy`: Restricts access to specific identity

#### ServiceEntry

```yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: openai-api
  namespace: agensys-demo-1
spec:
  hosts:
  - "api.openai.com"
  ports:
  - number: 443
    name: https
    protocol: TLS
  resolution: DNS
  location: MESH_EXTERNAL
```

#### AuthorizationPolicy

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-pr-agent-to-openai
  namespace: agensys-demo-1
spec:
  selector:
    matchLabels:
      istio: ztunnel
  action: ALLOW
  rules:
  - from:
    - source:
        principals: ["cluster.local/ns/agensys-demo-1/sa/pr-agent"]
    to:
    - operation:
        hosts: ["api.openai.com"]
        ports: ["443"]
```

### YAML Component Breakdown

#### ServiceEntry Components

| Component | Value | Purpose |
|-----------|-------|---------|
| `kind` | `ServiceEntry` | Defines external service |
| `spec.hosts` | `["api.openai.com"]` | External hostname |
| `spec.ports[0].number` | `443` | HTTPS port |
| `spec.ports[0].protocol` | `TLS` | Encrypted traffic |
| `spec.resolution` | `DNS` | Use DNS to resolve hostname |
| `spec.location` | `MESH_EXTERNAL` | Service is outside the mesh |

#### AuthorizationPolicy Components

| Component | Value | Purpose |
|-----------|-------|---------|
| `spec.selector.matchLabels.istio` | `ztunnel` | Apply to ztunnel (egress enforcement) |
| `spec.rules[0].from[0].source.principals` | `["cluster.local/ns/agensys-demo-1/sa/pr-agent"]` | Only PR Agent allowed |
| `spec.rules[0].to[0].operation.hosts` | `["api.openai.com"]` | Only OpenAI API |
| `spec.rules[0].to[0].operation.ports` | `["443"]` | Only HTTPS |

### How It Works

**ServiceEntry: Making External Services Visible**
- By default, mesh-enabled pods cannot access external services
- ServiceEntry "registers" external hosts with the mesh
- Allows ztunnel to route traffic to external destinations
- Does NOT grant access (that's the AuthorizationPolicy's job)

**Why ServiceEntry is Required**
Without ServiceEntry:
```
PR Agent → (tries to connect) → api.openai.com
    ↓
Ztunnel: "I don't know this host" → DNS fails or connection dropped
```

With ServiceEntry:
```
PR Agent → api.openai.com (request)
    ↓
Ztunnel: "I know this host, checking authorization..."
    ↓
AuthorizationPolicy: Check if PR Agent is allowed
    ↓
✅ Allowed → Establish TLS connection to api.openai.com
```

**Selector: `istio: ztunnel`**
- This policy applies to ztunnel, not to destination pods
- Why? Because OpenAI is external - there's no pod to apply policy to
- Ztunnel on the **source node** (where PR Agent runs) enforces egress policy
- This is different from internal policies (which apply to destination)

**Egress vs Ingress Enforcement**
- **Ingress** (internal): Policy applied to destination pod's ztunnel
- **Egress** (external): Policy applied to source pod's ztunnel
- This policy is egress because destination is outside mesh

### Security Impact

✅ **Enforces:**
- PR Agent can ONLY call OpenAI (no other LLMs)
- No arbitrary internet access
- Cost control (only authorized LLM usage)
- Architectural decision enforced at network layer

✅ **Defense-in-Depth:**
- Even if OpenAI API key is compromised
- Attacker cannot use it from Summary Agent or MCP servers
- Network policy provides second layer of security

❌ **Prevents:**
- PR Agent → Anthropic API (blocked)
- PR Agent → Google.com (blocked)
- PR Agent → GitHub API (blocked)
- Summary Agent → OpenAI API (blocked - wrong SPIFFE ID)

### Testing

#### Kubernetes Perspective

**Check ServiceEntry exists:**
```bash
kubectl get serviceentry openai-api -n agensys-demo-1
```

Expected:
```
NAME         HOSTS                 LOCATION        RESOLUTION   AGE
openai-api   ["api.openai.com"]    MESH_EXTERNAL   DNS          5m
```

**Check AuthorizationPolicy exists:**
```bash
kubectl get authorizationpolicy allow-pr-agent-to-openai -n agensys-demo-1
```

**Verify policy targets ztunnel:**
```bash
kubectl get authorizationpolicy allow-pr-agent-to-openai -n agensys-demo-1 -o jsonpath='{.spec.selector.matchLabels}'
```

Expected: `{"istio":"ztunnel"}`

**Verify allowed principal:**
```bash
kubectl get authorizationpolicy allow-pr-agent-to-openai -n agensys-demo-1 -o jsonpath='{.spec.rules[0].from[0].source.principals[0]}'
```

Expected: `cluster.local/ns/agensys-demo-1/sa/pr-agent`

**Check PR Agent ServiceAccount:**
```bash
kubectl get pods -n agensys-demo-1 -l app=pr-agent -o jsonpath='{.items[0].spec.serviceAccountName}'
```

Expected: `pr-agent`

#### Curl Testing

**Test 1: PR Agent → OpenAI API (should SUCCEED)**
```bash
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -v -m 10 https://api.openai.com/v1/models
```

**Expected Result:**
```
< HTTP/1.1 401 Unauthorized
< Content-Type: application/json
{
  "error": {
    "message": "You didn't provide an API key...",
    "type": "invalid_request_error"
  }
}
```

**Analysis:**
- Connection **succeeded** (network policy allowed it)
- Got HTTP 401 because no API key was provided
- This proves the network policy is working correctly
- The 401 is expected - we're testing network access, not API auth

**What's happening:**
1. PR Agent sends HTTPS request to api.openai.com:443
2. Ztunnel on PR Agent's node intercepts egress traffic
3. Checks ServiceEntry: "api.openai.com is a known external service" ✅
4. Checks AuthorizationPolicy `allow-pr-agent-to-openai`
5. Verifies source principal: `cluster.local/ns/agensys-demo-1/sa/pr-agent` ✅
6. Verifies destination host: `api.openai.com` ✅
7. Verifies destination port: `443` ✅
8. Allows connection through
9. Establishes TLS connection to OpenAI
10. OpenAI responds with 401 (missing API key)

**Test 2: PR Agent → Anthropic API (should FAIL)**
```bash
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -v -m 10 https://api.anthropic.com/v1/messages
```

**Expected Result:**
```
curl: (6) Could not resolve host: api.anthropic.com
```

OR (if DNS resolves):
```
curl: (28) Connection timed out after 10000 milliseconds
```

**Why it fails:**
1. PR Agent attempts connection to api.anthropic.com
2. Ztunnel checks ServiceEntry list
3. No ServiceEntry for api.anthropic.com exists for PR Agent
4. OR: ServiceEntry exists but AuthorizationPolicy doesn't allow PR Agent
5. Connection blocked at ztunnel (egress enforcement)

**Test 3: PR Agent → GitHub API (should FAIL)**
```bash
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -v -m 10 https://api.github.com
```

**Expected Result:**
```
curl: (28) Connection timed out
```

**Test 4: Summary Agent → OpenAI API (should FAIL - wrong SPIFFE ID)**
```bash
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- \
  curl -v -m 10 https://api.openai.com/v1/models
```

**Expected Result:**
```
curl: (28) Connection timed out
```

**Why it fails:**
1. Summary Agent attempts connection to api.openai.com
2. Ztunnel on Summary Agent's node checks policies
3. Finds ServiceEntry for api.openai.com (service is known)
4. Checks AuthorizationPolicy `allow-pr-agent-to-openai`
5. Required principal: `cluster.local/ns/agensys-demo-1/sa/pr-agent`
6. Actual principal: `cluster.local/ns/agensys-demo-1/sa/summary-agent`
7. Does NOT match ❌
8. Connection blocked

This proves LLM isolation - each agent can only access its designated LLM.

---

## Policy 4: Summary Agent to Anthropic

### File: `04-summary-agent-policies.yaml`

### Purpose
Permits the Executive Summary Agent to communicate with the Anthropic API (Claude) for generating executive summaries. This is the agent's sole external connection.

### YAML Structure

Identical structure to Policy 3, but for Anthropic:

```yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: anthropic-api
  namespace: agensys-demo-1
spec:
  hosts:
  - "api.anthropic.com"
  ports:
  - number: 443
    name: https
    protocol: TLS
  resolution: DNS
  location: MESH_EXTERNAL

---
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-summary-agent-to-anthropic
  namespace: agensys-demo-1
spec:
  selector:
    matchLabels:
      istio: ztunnel
  action: ALLOW
  rules:
  - from:
    - source:
        principals: ["cluster.local/ns/agensys-demo-1/sa/summary-agent"]
    to:
    - operation:
        hosts: ["api.anthropic.com"]
        ports: ["443"]
```

### Key Differences from Policy 3

| Component | Policy 3 (PR Agent) | Policy 4 (Summary Agent) |
|-----------|---------------------|--------------------------|
| ServiceEntry host | `api.openai.com` | `api.anthropic.com` |
| Principal | `sa/pr-agent` | `sa/summary-agent` |
| Purpose | Code analysis | Executive summaries |

### Architecture Decision

**Why separate LLMs?**
- GPT-4: Excellent for code analysis, debugging, technical details
- Claude: Excellent for executive summaries, narrative synthesis
- Network policies enforce this architectural separation
- Prevents accidental or malicious LLM mixing

### Testing

#### Kubernetes Perspective

**Check both resources exist:**
```bash
kubectl get serviceentry anthropic-api -n agensys-demo-1
kubectl get authorizationpolicy allow-summary-agent-to-anthropic -n agensys-demo-1
```

**Verify Summary Agent ServiceAccount:**
```bash
kubectl get pods -n agensys-demo-1 -l app=executive-summary-agent -o jsonpath='{.items[0].spec.serviceAccountName}'
```

Expected: `summary-agent`

#### Curl Testing

**Test 1: Summary Agent → Anthropic API (should SUCCEED)**
```bash
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- \
  curl -v -m 10 https://api.anthropic.com/v1/messages
```

**Expected Result:**
```
< HTTP/1.1 400 Bad Request
< Content-Type: application/json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "missing required 'x-api-key' header"
  }
}
```

**Analysis:**
- Connection succeeded (network policy worked)
- Got 400 due to missing x-api-key header
- This is expected - we're testing network access, not API auth

**Test 2: Summary Agent → OpenAI API (should FAIL)**
```bash
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- \
  curl -v -m 10 https://api.openai.com/v1/models
```

**Expected Result:**
```
curl: (28) Connection timed out
```

This proves LLM separation - Summary Agent cannot access OpenAI.

---

## Policy 5: MCP Scanning to Semgrep

### File: `05-mcp-scanning-policies.yaml`

### Purpose
Authorizes the MCP Code Scanning Server to connect to Semgrep Cloud Platform for vulnerability detection.

### YAML Structure

```yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: semgrep-cloud
  namespace: agensys-demo-1
spec:
  hosts:
  - "semgrep.dev"
  ports:
  - number: 443
    name: https
    protocol: TLS
  resolution: DNS
  location: MESH_EXTERNAL

---
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-mcp-scanning-to-semgrep
  namespace: agensys-demo-1
spec:
  selector:
    matchLabels:
      istio: ztunnel
  action: ALLOW
  rules:
  - from:
    - source:
        principals: ["cluster.local/ns/agensys-demo-1/sa/mcp-code-scanning"]
    to:
    - operation:
        hosts: ["semgrep.dev"]
        ports: ["443"]
```

### MCP Architecture Note

**What is an MCP Server?**
- MCP = Model Context Protocol
- Purpose-built adapter between agents and external tools
- Each MCP server should access ONLY its designated tool
- This policy enforces single-purpose access

**Why this matters:**
- If MCP Scanning server is compromised, blast radius is limited to Semgrep
- Cannot pivot to GitHub, LLMs, or other services
- Clear audit trail: all Semgrep access comes from this component

### Testing

#### Kubernetes Perspective

**Verify MCP Scanning ServiceAccount:**
```bash
kubectl get pods -n agensys-demo-1 -l app=mcp-code-scanning -o jsonpath='{.items[0].spec.serviceAccountName}'
```

Expected: `mcp-code-scanning`

#### Curl Testing

**Test 1: MCP Scanning → Semgrep (should SUCCEED)**
```bash
kubectl exec -n agensys-demo-1 deploy/mcp-code-scanning -- \
  curl -v -m 10 https://semgrep.dev
```

**Expected Result:**
```
< HTTP/1.1 200 OK
< Content-Type: text/html
<!DOCTYPE html>
<html>
  <head><title>Semgrep</title></head>
...
```

**Test 2: MCP Scanning → OpenAI (should FAIL)**
```bash
kubectl exec -n agensys-demo-1 deploy/mcp-code-scanning -- \
  curl -v -m 10 https://api.openai.com
```

**Expected Result:**
```
curl: (28) Connection timed out
```

**Test 3: MCP Scanning → GitHub (should FAIL)**
```bash
kubectl exec -n agensys-demo-1 deploy/mcp-code-scanning -- \
  curl -v -m 10 https://api.github.com
```

**Expected Result:**
```
curl: (28) Connection timed out
```

This proves tool isolation - MCP servers can only access their designated tool.

---

## Policy 6: GitHub MCP to GitHub API

### File: `06-mcp-github-policies.yaml`

### Purpose
Grants the GitHub MCP Server permission to communicate with GitHub's API for posting PR comments. This is the single point of GitHub integration.

### YAML Structure

```yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: github-api
  namespace: agensys-demo-1
spec:
  hosts:
  - "api.github.com"
  ports:
  - number: 443
    name: https
    protocol: TLS
  resolution: DNS
  location: MESH_EXTERNAL

---
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-github-mcp-to-github
  namespace: agensys-demo-1
spec:
  selector:
    matchLabels:
      istio: ztunnel
  action: ALLOW
  rules:
  - from:
    - source:
        principals: ["cluster.local/ns/agensys-demo-1/sa/github-mcp-server"]
    to:
    - operation:
        hosts: ["api.github.com"]
        ports: ["443"]
```

### Compliance Note

**Centralized GitHub Access**
- Many organizations require external API access be centralized and logged
- GitHub MCP Server becomes the single chokepoint
- All GitHub interactions flow through one auditable component
- Agents cannot bypass workflow to post directly to PRs

### Testing

#### Kubernetes Perspective

**Verify GitHub MCP ServiceAccount:**
```bash
kubectl get pods -n agensys-demo-1 -l app=github-mcp-server -o jsonpath='{.items[0].spec.serviceAccountName}'
```

Expected: `github-mcp-server`

#### Curl Testing

**Test 1: GitHub MCP → GitHub API (should SUCCEED)**
```bash
kubectl exec -n agensys-demo-1 deploy/github-mcp-server -- \
  curl -v -m 10 https://api.github.com
```

**Expected Result:**
```
< HTTP/1.1 200 OK
< Content-Type: application/json
{
  "current_user_url": "https://api.github.com/user",
  "authorizations_url": "https://api.github.com/authorizations",
  ...
}
```

**Test 2: GitHub MCP → Semgrep (should FAIL)**
```bash
kubectl exec -n agensys-demo-1 deploy/github-mcp-server -- \
  curl -v -m 10 https://semgrep.dev
```

**Expected Result:**
```
curl: (28) Connection timed out
```

**Test 3: GitHub MCP → OpenAI (should FAIL)**
```bash
kubectl exec -n agensys-demo-1 deploy/github-mcp-server -- \
  curl -v -m 10 https://api.openai.com
```

**Expected Result:**
```
curl: (28) Connection timed out
```

---

## Understanding Key YAML Components

### AuthorizationPolicy Structure

All AuthorizationPolicies follow this structure:

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: <policy-name>
  namespace: <namespace>
spec:
  selector:              # Which workloads this applies to
    matchLabels:
      <label-key>: <label-value>
  action: ALLOW | DENY   # What to do when rule matches
  rules:                 # Conditions that must be met
  - from:                # Source requirements
    - source:
        principals: []   # SPIFFE identities
        namespaces: []   # Source namespaces
    to:                  # Destination requirements
    - operation:
        hosts: []        # Destination hosts
        ports: []        # Destination ports
        methods: []      # HTTP methods
        paths: []        # URL paths
```

### Critical Components Explained

#### 1. Selector

**Purpose:** Determines which pods this policy applies to

**Two types:**
- **Empty selector (`{}`)**:  Applies to ALL workloads in namespace (default-deny)
- **Label selector**: Applies only to pods with matching labels

**Example:**
```yaml
selector:
  matchLabels:
    app: pr-agent
```
This applies ONLY to pods labeled `app=pr-agent`.

**How to verify:**
```bash
kubectl get pods -n agensys-demo-1 -l app=pr-agent --show-labels
```

#### 2. Action

**Purpose:** What to do when rules match

**Values:**
- `ALLOW`: Create exception to default-deny
- `DENY`: Explicitly block (overrides ALLOW policies)
- `CUSTOM`: Use external authorizer (advanced)

**Default:** If no action specified, defaults to DENY

#### 3. Principals (SPIFFE Identities)

**Format:** `cluster.local/ns/<namespace>/sa/<service-account>`

**Example:** `cluster.local/ns/agensys-demo-1/sa/orchestrator-agent`

**Components:**
- `cluster.local`: Trust domain (cluster-wide)
- `ns/agensys-demo-1`: Namespace
- `sa/orchestrator-agent`: ServiceAccount name

**How it works:**
1. Pod is assigned a ServiceAccount
2. Istio CA issues x509 certificate with SPIFFE ID in SAN field
3. Ztunnel presents this certificate during mTLS handshake
4. Receiving ztunnel verifies certificate and extracts SPIFFE ID
5. Authorization policy checks if SPIFFE ID is allowed

**Why this is secure:**
- Certificate-based (not IP-based)
- Cryptographically verified
- Cannot be spoofed
- Rotated automatically

**How to check a pod's ServiceAccount:**
```bash
kubectl get pod <pod-name> -n agensys-demo-1 -o jsonpath='{.spec.serviceAccountName}'
```

#### 4. Ports

**Purpose:** Restrict which ports can be accessed

**Format:** Array of strings: `["8080", "3000", "443"]`

**Why strings, not integers?**
- Kubernetes YAML compatibility
- Allows named ports: `["http", "https"]`

**Example:**
```yaml
to:
- operation:
    ports: ["8080"]
```

Only port 8080 is accessible, all other ports blocked.

#### 5. Methods (HTTP only)

**Purpose:** Restrict HTTP methods

**Values:** `["GET", "POST", "PUT", "DELETE", "PATCH"]`

**Example:**
```yaml
to:
- operation:
    methods: ["POST"]
```

Only POST requests allowed, GET/PUT/DELETE blocked.

**Note:** Only works with HTTP/HTTPS traffic, not TCP.

#### 6. Hosts (for External Services)

**Purpose:** Specify which external hosts can be accessed

**Used in:** Egress policies (external API access)

**Example:**
```yaml
to:
- operation:
    hosts: ["api.openai.com"]
    ports: ["443"]
```

Only api.openai.com on port 443 is accessible.

### ServiceEntry Structure

```yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: <service-name>
  namespace: <namespace>
spec:
  hosts:                 # External hostnames
  - "external.api.com"
  ports:                 # Ports to access
  - number: 443
    name: https
    protocol: TLS
  resolution: DNS        # How to resolve hostname
  location: MESH_EXTERNAL  # External to mesh
```

### ServiceEntry Components Explained

#### 1. Hosts

**Purpose:** External hostnames to make accessible

**Format:** Array of strings: `["api.openai.com", "api.github.com"]`

**Wildcards supported:** `["*.google.com"]`

**Example:**
```yaml
hosts:
- "api.openai.com"
```

#### 2. Ports

**Purpose:** Define which ports can be used

**Components:**
- `number`: Port number (443, 80, 8080)
- `name`: Human-readable name (https, http)
- `protocol`: TLS, HTTP, TCP, etc.

**Example:**
```yaml
ports:
- number: 443
  name: https
  protocol: TLS
```

#### 3. Resolution

**Purpose:** How to resolve the hostname

**Values:**
- `DNS`: Use DNS to resolve (most common for external APIs)
- `STATIC`: Use provided IP addresses
- `NONE`: Assume mesh already knows how to route

**Example:**
```yaml
resolution: DNS
```

Ztunnel will use DNS to resolve api.openai.com to an IP address.

#### 4. Location

**Purpose:** Where is this service located

**Values:**
- `MESH_EXTERNAL`: Outside the mesh (external APIs)
- `MESH_INTERNAL`: Inside the mesh (rarely used in ServiceEntry)

**Example:**
```yaml
location: MESH_EXTERNAL
```

This tells Istio the service is external, don't look for pods.

### Combining ServiceEntry + AuthorizationPolicy

**Why both are needed for external access:**

1. **ServiceEntry** says: "This external host exists and here's how to reach it"
2. **AuthorizationPolicy** says: "Only this specific workload can access it"

**Without ServiceEntry:**
```
PR Agent → api.openai.com
    ↓
Ztunnel: "Unknown host" → Connection fails
```

**With ServiceEntry, without AuthorizationPolicy:**
```
PR Agent → api.openai.com
    ↓
Ztunnel: "Known host, checking if anyone can access..."
    ↓
No specific allow policy → Falls back to default-deny → Blocked
```

**With both:**
```
PR Agent → api.openai.com
    ↓
Ztunnel: "Known host (ServiceEntry), checking authorization..."
    ↓
AuthorizationPolicy: PR Agent is allowed → ✅ Connection proceeds
```

---

## Testing Methodology

### Two-Level Testing Approach

#### Level 1: Kubernetes Resource Validation
Verify policies are correctly applied and configured

#### Level 2: Runtime Connectivity Testing
Verify policies actually enforce the intended behavior

### Level 1: Kubernetes Resource Validation

#### Step 1: Verify All Policies Exist

```bash
kubectl get authorizationpolicies -n agensys-demo-1
```

**Expected output:**
```
NAME                                  AGE
default-deny-all                      5m
allow-orchestrator-to-pr-agent        5m
allow-orchestrator-to-mcp-scanning    5m
allow-orchestrator-to-summary-agent   5m
allow-orchestrator-to-github-mcp      5m
allow-pr-agent-to-openai              5m
allow-summary-agent-to-anthropic      5m
allow-mcp-scanning-to-semgrep         5m
allow-github-mcp-to-github            5m
```

Should have exactly 9 policies.

#### Step 2: Verify ServiceEntries Exist

```bash
kubectl get serviceentries -n agensys-demo-1
```

**Expected output:**
```
NAME            HOSTS                  LOCATION        RESOLUTION   AGE
openai-api      ["api.openai.com"]     MESH_EXTERNAL   DNS          5m
anthropic-api   ["api.anthropic.com"]  MESH_EXTERNAL   DNS          5m
semgrep-cloud   ["semgrep.dev"]        MESH_EXTERNAL   DNS          5m
github-api      ["api.github.com"]     MESH_EXTERNAL   DNS          5m
```

Should have exactly 4 ServiceEntries.

#### Step 3: Verify Namespace is in Ambient Mode

```bash
kubectl get namespace agensys-demo-1 -o jsonpath='{.metadata.labels.istio\.io/dataplane-mode}'
```

**Expected output:**
```
ambient
```

#### Step 4: Verify Ztunnel is Running

```bash
kubectl get pods -n istio-system -l app=ztunnel
```

**Expected:**
At least one ztunnel pod per node, all Running.

#### Step 5: Verify ServiceAccounts Match Policy Principals

```bash
# Check Orchestrator
kubectl get pods -n agensys-demo-1 -l app=orchestrator-agent -o jsonpath='{.items[0].spec.serviceAccountName}'

# Check PR Agent
kubectl get pods -n agensys-demo-1 -l app=pr-agent -o jsonpath='{.items[0].spec.serviceAccountName}'

# Check Summary Agent
kubectl get pods -n agensys-demo-1 -l app=executive-summary-agent -o jsonpath='{.items[0].spec.serviceAccountName}'

# Check MCP Scanning
kubectl get pods -n agensys-demo-1 -l app=mcp-code-scanning -o jsonpath='{.items[0].spec.serviceAccountName}'

# Check GitHub MCP
kubectl get pods -n agensys-demo-1 -l app=github-mcp-server -o jsonpath='{.items[0].spec.serviceAccountName}'
```

**Expected:**
- orchestrator-agent
- pr-agent
- summary-agent
- mcp-code-scanning
- github-mcp-server

#### Step 6: Verify Pod Labels Match Policy Selectors

```bash
# For internal policies (e.g., Orchestrator → PR Agent)
kubectl get pods -n agensys-demo-1 -l app=pr-agent --show-labels
```

**Expected:** Should see `app=pr-agent` in labels.

#### Step 7: Check Policy Configuration Details

```bash
# Example: Verify PR Agent can access OpenAI
kubectl get authorizationpolicy allow-pr-agent-to-openai -n agensys-demo-1 -o yaml
```

**Verify:**
- `spec.selector.matchLabels.istio: ztunnel` (for egress policies)
- `spec.action: ALLOW`
- Correct principals in `spec.rules[0].from[0].source.principals`
- Correct hosts in `spec.rules[0].to[0].operation.hosts`

### Level 2: Runtime Connectivity Testing

#### Test Category 1: Allowed Internal Connections

These should all return HTTP 200 OK:

```bash
# Orchestrator → PR Agent
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- \
  curl -s -m 5 http://pr-agent:8080/health

# Orchestrator → MCP Scanning
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- \
  curl -s -m 5 http://mcp-code-scanning:3000/health

# Orchestrator → Summary Agent
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- \
  curl -s -m 5 http://executive-summary-agent:8080/health

# Orchestrator → GitHub MCP
kubectl exec -n agensys-demo-1 deploy/orchestrator-agent -- \
  curl -s -m 5 http://github-mcp-server:3000/health
```

**Expected for all:** HTTP 200 OK with JSON response

#### Test Category 2: Allowed External Connections

These should connect (may get auth errors, but connection succeeds):

```bash
# PR Agent → OpenAI
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -s -m 10 https://api.openai.com/v1/models

# Summary Agent → Anthropic
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- \
  curl -s -m 10 https://api.anthropic.com/v1/messages

# MCP Scanning → Semgrep
kubectl exec -n agensys-demo-1 deploy/mcp-code-scanning -- \
  curl -s -m 10 https://semgrep.dev

# GitHub MCP → GitHub
kubectl exec -n agensys-demo-1 deploy/github-mcp-server -- \
  curl -s -m 10 https://api.github.com
```

**Expected:**
- OpenAI: 401 Unauthorized (connection worked, auth failed)
- Anthropic: 400 Bad Request (connection worked, missing headers)
- Semgrep: 200 OK (public homepage)
- GitHub: 200 OK (API root)

#### Test Category 3: Denied Inter-Agent Connections

These should all timeout:

```bash
# PR Agent → Summary Agent (should be blocked)
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -s -m 5 http://executive-summary-agent:8080/health

# PR Agent → GitHub MCP (should be blocked)
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -s -m 5 http://github-mcp-server:3000/health

# Summary Agent → PR Agent (should be blocked)
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- \
  curl -s -m 5 http://pr-agent:8080/health

# Summary Agent → GitHub MCP (should be blocked)
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- \
  curl -s -m 5 http://github-mcp-server:3000/health
```

**Expected for all:** `curl: (28) Connection timed out after 5000 milliseconds`

#### Test Category 4: Denied Cross-LLM Access

These should all timeout:

```bash
# PR Agent → Anthropic (should be blocked)
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -s -m 10 https://api.anthropic.com

# Summary Agent → OpenAI (should be blocked)
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- \
  curl -s -m 10 https://api.openai.com
```

**Expected:** Connection timeout

#### Test Category 5: Denied MCP Cross-Tool Access

These should all timeout:

```bash
# MCP Scanning → GitHub (should be blocked)
kubectl exec -n agensys-demo-1 deploy/mcp-code-scanning -- \
  curl -s -m 10 https://api.github.com

# MCP Scanning → OpenAI (should be blocked)
kubectl exec -n agensys-demo-1 deploy/mcp-code-scanning -- \
  curl -s -m 10 https://api.openai.com

# GitHub MCP → Semgrep (should be blocked)
kubectl exec -n agensys-demo-1 deploy/github-mcp-server -- \
  curl -s -m 10 https://semgrep.dev

# GitHub MCP → OpenAI (should be blocked)
kubectl exec -n agensys-demo-1 deploy/github-mcp-server -- \
  curl -s -m 10 https://api.openai.com
```

**Expected:** Connection timeout

#### Test Category 6: Denied Arbitrary External Access

These should all timeout:

```bash
# PR Agent → Google (should be blocked)
kubectl exec -n agensys-demo-1 deploy/pr-agent -- \
  curl -s -m 10 https://google.com

# Summary Agent → Example.com (should be blocked)
kubectl exec -n agensys-demo-1 deploy/executive-summary-agent -- \
  curl -s -m 10 https://example.com

# MCP Scanning → Random site (should be blocked)
kubectl exec -n agensys-demo-1 deploy/mcp-code-scanning -- \
  curl -s -m 10 https://microsoft.com
```

**Expected:** Connection timeout

### Monitoring Policy Enforcement

#### View Real-Time Policy Decisions

```bash
# Watch ztunnel logs for RBAC decisions
kubectl logs -n istio-system -l app=ztunnel -f | grep -E "(ALLOW|DENY|RBAC)"
```

Run tests in another terminal and watch decisions happen live.

#### View Denied Connections

```bash
# See recent policy violations
kubectl logs -n istio-system -l app=ztunnel --tail=100 | grep RBAC_ACCESS_DENIED
```

**Example output:**
```
[2025-01-15T10:45:12] RBAC_ACCESS_DENIED: source=pr-agent destination=executive-summary-agent
[2025-01-15T10:45:18] RBAC_ACCESS_DENIED: source=pr-agent destination=api.anthropic.com
```

#### Query Policy Metrics

```bash
# Get authorization metrics from istiod
kubectl exec -n istio-system deploy/istiod -- \
  curl -s localhost:15014/metrics | grep istio_authorization

# Get ztunnel connection metrics
kubectl exec -n istio-system -l app=ztunnel -- \
  curl -s localhost:15020/stats/prometheus | grep istio_tcp_connections
```

### Automated Test Script

Use the provided `connectivity-test.sh` script for comprehensive validation:

```bash
chmod +x connectivity-test.sh
./connectivity-test.sh
```

This script tests all 50+ connectivity scenarios and provides a pass/fail summary.

---

## Summary

This technical documentation provides complete details on:

✅ **6 policies** with detailed YAML breakdowns
✅ **Key components** explained (selectors, principals, ports, etc.)
✅ **How each policy works** at the network level
✅ **Complete testing methodology** from Kubernetes and curl perspectives
✅ **Expected results** for all test scenarios
✅ **Monitoring and troubleshooting** guidance

Use this document as a reference when:
- Implementing Zero-Trust policies in your own cluster
- Debugging policy issues
- Understanding Istio Ambient Mode security
- Training team members on network security architecture
- Explaining policies to security auditors or compliance teams
