# Zero-Trust Architecture Diagrams

This document contains visual diagrams explaining the Istio Ambient Mode architecture, ztunnel operation, HBONE tunneling, and policy enforcement for the autonomous code review system.

## Diagram Summary

| # | Diagram Name | Purpose |
|---|--------------|---------|
| 1 | High-Level System Architecture | Overall system with all components |
| 2 | Ztunnel Architecture | How ztunnel operates on a node |
| 3 | HBONE Tunnel | HTTP/2 CONNECT tunnel between nodes |
| 4 | Traffic Flow with Policies | Complete request flow with decisions |
| 5 | Default Deny Policy | Effect of global deny-all |
| 6 | Orchestrator Policies | Hub-and-spoke communication pattern |
| 7 | External API Policies | Egress control to external services |
| 8 | Complete Policy Matrix | All allowed/blocked paths |
| 9 | mTLS Certificate Flow | Certificate issuance and rotation |
| 10 | SPIFFE Authorization | Identity extraction and matching |
| 11 | ServiceEntry + AuthZ | Why both are needed |
| 12 | Real Traffic Example | PR Agent → OpenAI (success) |
| 13 | Policy Violation Example | PR Agent → Anthropic (blocked) |

---

## Color Legend

Throughout these diagrams, consistent colors are used:

- 🟦 **Blue** - Control plane components (Istiod, CA)
- 🟧 **Orange** - Data plane components (Ztunnel)
- 🟪 **Purple** - Application agents
- 🟩 **Green** - External services / Allowed connections
- 🟥 **Red** - Blocked/denied connections
- 🟨 **Yellow** - Policy decision points

---

## Diagram 1: High-Level System Architecture

```mermaid
graph TB
    subgraph "GitHub"
        GH[GitHub Repository]
    end

    subgraph "Kubernetes Cluster - Namespace: agensys-demo-1"
        subgraph "Control Plane - istio-system"
            ISTIOD[Istiod<br/>Control Plane]
            CA[Certificate Authority<br/>Issues mTLS Certs]
        end

        subgraph "Node 1"
            ZT1[Ztunnel<br/>DaemonSet]
            ORCH[Orchestrator<br/>Agent]
            PRA[PR Agent]
        end

        subgraph "Node 2"
            ZT2[Ztunnel<br/>DaemonSet]
            SUM[Summary<br/>Agent]
            MCPS[MCP Scanning<br/>Server]
        end

        subgraph "Node 3"
            ZT3[Ztunnel<br/>DaemonSet]
            MCPG[GitHub MCP<br/>Server]
        end
    end

    subgraph "External Services"
        OPENAI[OpenAI API<br/>GPT-4]
        ANTHROPIC[Anthropic API<br/>Claude]
        SEMGREP[Semgrep Cloud]
        GHAPI[GitHub API]
    end

    GH -->|Webhook| ORCH
    
    ORCH -.->|mTLS via ZT1| ZT1
    ZT1 -.->|HBONE| ZT2
    ZT1 -.->|HBONE| ZT3
    ZT2 -->|Local| SUM
    ZT2 -->|Local| MCPS
    ZT3 -->|Local| MCPG

    PRA -->|Policy Check| ZT1
    ZT1 -->|Allowed| OPENAI

    SUM -->|Policy Check| ZT2
    ZT2 -->|Allowed| ANTHROPIC

    MCPS -->|Policy Check| ZT2
    ZT2 -->|Allowed| SEMGREP

    MCPG -->|Policy Check| ZT3
    ZT3 -->|Allowed| GHAPI

    ISTIOD -.->|xDS Config| ZT1
    ISTIOD -.->|xDS Config| ZT2
    ISTIOD -.->|xDS Config| ZT3
    
    CA -.->|Issues Certs| ZT1
    CA -.->|Issues Certs| ZT2
    CA -.->|Issues Certs| ZT3

    classDef controlPlane fill:#e1f5ff,stroke:#01579b,stroke-width:2px,color:#000
    classDef dataPlane fill:#fff3e0,stroke:#e65100,stroke-width:2px,color:#000
    classDef agent fill:#f3e5f5,stroke:#4a148c,stroke-width:2px,color:#000
    classDef external fill:#e8f5e9,stroke:#1b5e20,stroke-width:2px,color:#000

    class ISTIOD,CA controlPlane
    class ZT1,ZT2,ZT3 dataPlane
    class ORCH,PRA,SUM,MCPS,MCPG agent
    class OPENAI,ANTHROPIC,SEMGREP,GHAPI,GH external
```

**Description:** This diagram shows the complete system architecture with Istio control plane (istiod + CA), ztunnel instances on each node, agent deployments, and external service connections.

---

## Diagram 2: Ztunnel Architecture and Responsibilities

```mermaid
graph TB
    subgraph "Kubernetes Node"
        subgraph "Pod 1 Network Namespace"
            APP1[Application<br/>PR Agent]
            IPTABLES1[iptables TPROXY Rules]
        end

        subgraph "Pod 2 Network Namespace"
            APP2[Application<br/>Summary Agent]
            IPTABLES2[iptables TPROXY Rules]
        end

        subgraph "Node Network Namespace"
            ZTUNNEL[Ztunnel Process<br/>Rust-based Proxy]
            
            subgraph "Ztunnel Components"
                LISTENER[Traffic Listener<br/>Port 15001, 15006]
                AUTHZ[Authorization<br/>Policy Engine]
                MTLS[mTLS Handler<br/>SPIFFE Auth]
                CERT[Certificate<br/>Store]
            end
        end

        subgraph "CNI Plugin"
            CNI[Istio CNI<br/>Network Setup]
        end
    end

    subgraph "Control Plane"
        ISTIOD2[Istiod]
        CA2[Certificate<br/>Authority]
    end

    APP1 -->|Send Traffic| IPTABLES1
    IPTABLES1 -.->|TPROXY Redirect| LISTENER
    
    APP2 -->|Send Traffic| IPTABLES2
    IPTABLES2 -.->|TPROXY Redirect| LISTENER

    LISTENER --> AUTHZ
    AUTHZ -->|Check Policy| MTLS
    MTLS -->|Use Cert| CERT
    MTLS -->|Forward if Allowed| APP2

    CNI -.->|Configure| IPTABLES1
    CNI -.->|Configure| IPTABLES2

    ISTIOD2 -.->|xDS: Policies<br/>Routes, Config| ZTUNNEL
    CA2 -.->|Issue/Rotate<br/>Certificates| CERT

    classDef app fill:#e1bee7,stroke:#4a148c,stroke-width:2px,color:#000
    classDef ztunnel fill:#fff3e0,stroke:#e65100,stroke-width:3px,color:#000
    classDef control fill:#e1f5ff,stroke:#01579b,stroke-width:2px,color:#000
    classDef network fill:#c8e6c9,stroke:#1b5e20,stroke-width:2px,color:#000

    class APP1,APP2 app
    class ZTUNNEL,LISTENER,AUTHZ,MTLS,CERT ztunnel
    class ISTIOD2,CA2 control
    class IPTABLES1,IPTABLES2,CNI network
```

**Description:** Shows how ztunnel operates as a node-level proxy, intercepting traffic via TPROXY, enforcing policies, and handling mTLS for multiple workloads on the same node.

---

## Diagram 3: HBONE (HTTP-Based Overlay Network) Tunnel

```mermaid
sequenceDiagram
    participant PRA as PR Agent<br/>(Node 1)
    participant ZT1 as Ztunnel Node 1<br/>(Source)
    participant ZT2 as Ztunnel Node 2<br/>(Destination)
    participant SUM as Summary Agent<br/>(Node 2)

    Note over PRA,SUM: Application wants to connect
    
    PRA->>ZT1: TCP connect to summary-agent:8080<br/>(intercepted by TPROXY)
    
    Note over ZT1: Extract SPIFFE ID from Pod<br/>spiffe://.../sa/pr-agent
    
    Note over ZT1: Check egress policy:<br/>Is pr-agent allowed to call summary-agent?
    
    ZT1->>ZT2: HTTP/2 CONNECT summary-agent:8080<br/>+ mTLS Handshake
    
    Note over ZT1,ZT2: mTLS Authentication<br/>ZT1 presents node cert<br/>ZT2 presents node cert<br/>Both verify via Istio CA
    
    ZT2-->>ZT1: 200 Connection Established
    
    Note over ZT1,ZT2: HBONE Tunnel Active<br/>Encrypted HTTP/2 Stream
    
    Note over ZT2: Check ingress policy:<br/>Is pr-agent allowed to access summary-agent?
    
    ZT2->>SUM: Forward traffic locally<br/>(if policy allows)
    
    SUM-->>ZT2: Response
    ZT2-->>ZT1: Response (through HBONE tunnel)
    ZT1-->>PRA: Response
    
    Note over PRA,SUM: Transparent to applications<br/>They see direct connection
```

**Description:** Sequence diagram showing how HBONE creates an mTLS tunnel between ztunnels using HTTP/2 CONNECT, making cross-node communication secure and transparent.

---

## Diagram 4: Traffic Flow with Policy Enforcement

```mermaid
flowchart TB
    START([Orchestrator sends request<br/>to Summary Agent])
    
    TPROXY{TPROXY intercepts<br/>in pod network namespace}
    
    ZT1_RCV[Ztunnel Node 1 receives<br/>redirected traffic]
    
    EXTRACT[Extract Source Identity:<br/>SPIFFE ID from pod's<br/>ServiceAccount]
    
    EGRESS_POL{Check Egress Policy:<br/>Can orchestrator-agent call<br/>summary-agent?}
    
    HBONE[Establish HBONE tunnel<br/>to Ztunnel Node 2<br/>with mTLS]
    
    ZT2_RCV[Ztunnel Node 2<br/>receives via HBONE]
    
    INGRESS_POL{Check Ingress Policy:<br/>Is orchestrator-agent allowed?}
    
    AUTHZ_CHECK[Check AuthorizationPolicy:<br/>allow-orchestrator-to-summary-agent<br/>Required: orchestrator-agent<br/>Actual: orchestrator-agent]
    
    DENY[❌ RBAC_ACCESS_DENIED<br/>Connection dropped<br/>Logged to ztunnel]
    
    ALLOW[✅ Policy Match<br/>Forward to Summary Agent]
    
    APP_RCV[Summary Agent<br/>receives request]
    
    RESPONSE[Response flows back<br/>through same tunnel]
    
    END([Orchestrator receives<br/>response or timeout])

    START --> TPROXY
    TPROXY --> ZT1_RCV
    ZT1_RCV --> EXTRACT
    EXTRACT --> EGRESS_POL
    
    EGRESS_POL -->|No matching<br/>allow policy| DENY
    EGRESS_POL -->|Policy exists| HBONE
    
    HBONE --> ZT2_RCV
    ZT2_RCV --> INGRESS_POL
    
    INGRESS_POL --> AUTHZ_CHECK
    AUTHZ_CHECK -->|No match| DENY
    AUTHZ_CHECK -->|Match| ALLOW
    
    ALLOW --> APP_RCV
    APP_RCV --> RESPONSE
    RESPONSE --> END
    
    DENY --> END

    classDef success fill:#c8e6c9,stroke:#2e7d32,stroke-width:2px,color:#000
    classDef failure fill:#ffcdd2,stroke:#c62828,stroke-width:2px,color:#000
    classDef check fill:#fff9c4,stroke:#f57f17,stroke-width:2px,color:#000
    classDef process fill:#e1f5fe,stroke:#0277bd,stroke-width:2px,color:#000

    class ALLOW,APP_RCV,RESPONSE success
    class DENY failure
    class EGRESS_POL,INGRESS_POL,AUTHZ_CHECK check
    class TPROXY,ZT1_RCV,EXTRACT,HBONE,ZT2_RCV process
```

**Description:** Flowchart showing complete traffic flow from source to destination with policy enforcement at both egress (source) and ingress (destination) points.

---

## Diagram 5: Default Deny Policy (Policy 1)

```mermaid
graph TB
    subgraph "Before Default Deny Policy"
        subgraph "Namespace: agensys-demo-1"
            A1[PR Agent]
            A2[Summary Agent]
            A3[MCP Scanning]
            A4[GitHub MCP]
            A5[Orchestrator]
        end
        
        A1 -.->|✅ Allowed| A2
        A1 -.->|✅ Allowed| A3
        A2 -.->|✅ Allowed| A4
        A3 -.->|✅ Allowed| A5
        A4 -.->|✅ Allowed| A1
        
        NOTE1[All traffic allowed by default<br/>No security enforcement]
    end

    POLICY[Apply Default Deny Policy:<br/>01-default-deny.yaml<br/>spec: emptyLeft]

    subgraph "After Default Deny Policy"
        subgraph "Namespace: agensys-demo-1"
            B1[PR Agent]
            B2[Summary Agent]
            B3[MCP Scanning]
            B4[GitHub MCP]
            B5[Orchestrator]
        end
        
        B1 -.->|❌ BLOCKED| B2
        B1 -.->|❌ BLOCKED| B3
        B2 -.->|❌ BLOCKED| B4
        B3 -.->|❌ BLOCKED| B5
        B4 -.->|❌ BLOCKED| B1
        
        NOTE2[All traffic DENIED<br/>Explicit allows required]
    end

    NOTE1 --> POLICY
    POLICY --> NOTE2

    classDef allowed fill:#c8e6c9,stroke:#2e7d32,stroke-width:2px,color:#000
    classDef denied fill:#ffcdd2,stroke:#c62828,stroke-width:2px,color:#000
    classDef policy fill:#fff9c4,stroke:#f57f17,stroke-width:3px,color:#000

    class A1,A2,A3,A4,A5 allowed
    class B1,B2,B3,B4,B5 denied
    class POLICY policy
```

**Description:** Shows the effect of the default-deny policy, transforming from "allow all" to "deny all" mode, requiring explicit allow policies for any communication.

---

## Diagram 6: Orchestrator Policies (Policy 2)

```mermaid
graph TB
    subgraph "Policy: 02-orchestrator-policies.yaml"
        subgraph "Namespace: agensys-demo-1"
            ORCH[Orchestrator Agent<br/>SPIFFE: .../sa/orchestrator-agent]
            
            PRA[PR Agent<br/>Port 8080<br/>Methods: POST]
            MCP_SCAN[MCP Scanning<br/>Port 3000]
            SUM[Summary Agent<br/>Port 8080<br/>Methods: POST]
            MCP_GH[GitHub MCP<br/>Port 3000]
        end
    end

    ORCH ==>|✅ ALLOW<br/>Policy 2a<br/>POST :8080| PRA
    ORCH ==>|✅ ALLOW<br/>Policy 2b<br/>:3000| MCP_SCAN
    ORCH ==>|✅ ALLOW<br/>Policy 2c<br/>POST :8080| SUM
    ORCH ==>|✅ ALLOW<br/>Policy 2d<br/>:3000| MCP_GH

    PRA -.->|❌ BLOCKED<br/>No policy| SUM
    PRA -.->|❌ BLOCKED<br/>No policy| MCP_SCAN
    SUM -.->|❌ BLOCKED<br/>No policy| MCP_GH
    MCP_SCAN -.->|❌ BLOCKED<br/>No policy| PRA

    NOTE[Only Orchestrator can<br/>communicate with agents<br/>All other inter-agent<br/>communication blocked]

    classDef orchestrator fill:#e1bee7,stroke:#4a148c,stroke-width:3px,color:#000
    classDef allowed fill:#c8e6c9,stroke:#2e7d32,stroke-width:2px,color:#000
    classDef blocked fill:#ffcdd2,stroke:#c62828,stroke-width:1px,stroke-dasharray: 5 5,color:#000

    class ORCH orchestrator
    class PRA,MCP_SCAN,SUM,MCP_GH allowed
```

**Description:** Shows how Orchestrator policies create a hub-and-spoke pattern where only the Orchestrator can communicate with agents, preventing lateral movement.

---

## Diagram 7: External API Policies (Policies 3-6)

```mermaid
graph LR
    subgraph "Namespace: agensys-demo-1"
        PRA[PR Agent<br/>SPIFFE: .../sa/pr-agent]
        SUM[Summary Agent<br/>SPIFFE: .../sa/summary-agent]
        MCP_SCAN[MCP Scanning<br/>SPIFFE: .../sa/mcp-code-scanning]
        MCP_GH[GitHub MCP<br/>SPIFFE: .../sa/github-mcp-server]
    end

    subgraph "External Services"
        OPENAI[OpenAI API<br/>api.openai.com:443]
        ANTHROPIC[Anthropic API<br/>api.anthropic.com:443]
        SEMGREP[Semgrep Cloud<br/>semgrep.dev:443]
        GHAPI[GitHub API<br/>api.github.com:443]
    end

    PRA ==>|✅ Policy 3<br/>ServiceEntry + AuthZ<br/>ALLOW pr-agent| OPENAI
    SUM ==>|✅ Policy 4<br/>ServiceEntry + AuthZ<br/>ALLOW summary-agent| ANTHROPIC
    MCP_SCAN ==>|✅ Policy 5<br/>ServiceEntry + AuthZ<br/>ALLOW mcp-code-scanning| SEMGREP
    MCP_GH ==>|✅ Policy 6<br/>ServiceEntry + AuthZ<br/>ALLOW github-mcp-server| GHAPI

    PRA -.->|❌ BLOCKED<br/>Wrong SPIFFE ID| ANTHROPIC
    SUM -.->|❌ BLOCKED<br/>Wrong SPIFFE ID| OPENAI
    MCP_SCAN -.->|❌ BLOCKED<br/>No ServiceEntry| GHAPI
    MCP_GH -.->|❌ BLOCKED<br/>No ServiceEntry| SEMGREP

    classDef agent fill:#e1bee7,stroke:#4a148c,stroke-width:2px,color:#000
    classDef external fill:#e8f5e9,stroke:#1b5e20,stroke-width:2px,color:#000
    classDef allowed stroke:#2e7d32,stroke-width:3px
    classDef blocked stroke:#c62828,stroke-width:1px,stroke-dasharray: 5 5

    class PRA,SUM,MCP_SCAN,MCP_GH agent
    class OPENAI,ANTHROPIC,SEMGREP,GHAPI external
```

**Description:** Illustrates egress policies where each agent/MCP server can only access its designated external service, enforced via ServiceEntry + AuthorizationPolicy combination.

---

## Diagram 8: Complete Policy Enforcement Matrix

```mermaid
graph TB
    subgraph "Complete Zero-Trust Policy Matrix"
        subgraph "Internal Communication"
            ORCH2[Orchestrator]
            
            subgraph "Agents & MCP Servers"
                PRA2[PR Agent]
                SUM2[Summary Agent]
                MCP_SCAN2[MCP Scanning]
                MCP_GH2[GitHub MCP]
            end
        end

        subgraph "External APIs"
            OPENAI2[OpenAI]
            ANTHROPIC2[Anthropic]
            SEMGREP2[Semgrep]
            GHAPI2[GitHub API]
        end
    end

    %% Orchestrator to internal services (ALLOWED)
    ORCH2 ==>|✅ POST :8080| PRA2
    ORCH2 ==>|✅ :3000| MCP_SCAN2
    ORCH2 ==>|✅ POST :8080| SUM2
    ORCH2 ==>|✅ :3000| MCP_GH2

    %% Agents to external APIs (ALLOWED)
    PRA2 ==>|✅ :443| OPENAI2
    SUM2 ==>|✅ :443| ANTHROPIC2
    MCP_SCAN2 ==>|✅ :443| SEMGREP2
    MCP_GH2 ==>|✅ :443| GHAPI2

    %% Inter-agent (BLOCKED)
    PRA2 -.->|❌| SUM2
    PRA2 -.->|❌| MCP_SCAN2
    SUM2 -.->|❌| PRA2
    SUM2 -.->|❌| MCP_GH2

    %% Wrong external API (BLOCKED)
    PRA2 -.->|❌| ANTHROPIC2
    SUM2 -.->|❌| OPENAI2
    MCP_SCAN2 -.->|❌| GHAPI2
    MCP_GH2 -.->|❌| SEMGREP2

    %% Default deny everything else
    PRA2 -.->|❌ Default Deny| INTERNET[Any Other<br/>Internet Host]
    SUM2 -.->|❌ Default Deny| INTERNET

    classDef orchestrator fill:#e1bee7,stroke:#4a148c,stroke-width:3px,color:#000
    classDef agent fill:#f3e5f5,stroke:#6a1b9a,stroke-width:2px,color:#000
    classDef external fill:#e8f5e9,stroke:#1b5e20,stroke-width:2px,color:#000
    classDef denied fill:#ffebee,stroke:#c62828,stroke-width:2px,color:#000

    class ORCH2 orchestrator
    class PRA2,SUM2,MCP_SCAN2,MCP_GH2 agent
    class OPENAI2,ANTHROPIC2,SEMGREP2,GHAPI2 external
    class INTERNET denied
```

**Description:** Complete matrix showing all allowed paths (solid green lines) and blocked paths (dashed red lines), demonstrating the comprehensive Zero-Trust security model.

---

## Diagram 9: mTLS Certificate Flow

```mermaid
sequenceDiagram
    participant POD as New Pod Starts<br/>(PR Agent)
    participant CNI as Istio CNI
    participant ZT as Ztunnel<br/>(on same node)
    participant ISTIOD as Istiod<br/>(Control Plane)
    participant CA as Certificate Authority<br/>(in Istiod)

    Note over POD: Pod created with<br/>ServiceAccount: pr-agent

    POD->>CNI: Pod network initialized
    CNI->>CNI: Setup TPROXY iptables rules<br/>in pod network namespace
    
    CNI->>ZT: Notify: New pod detected<br/>NS: agensys-demo-1<br/>SA: pr-agent
    
    ZT->>CA: CSR: Certificate Signing Request<br/>SPIFFE ID: spiffe://.../sa/pr-agent<br/>Node: node-1
    
    Note over CA: Verify request:<br/>- Node identity valid?<br/>- ServiceAccount exists?<br/>- Node hosts this pod?
    
    CA->>CA: Generate X.509 Certificate<br/>Subject: CN=pr-agent<br/>SAN: spiffe://.../sa/pr-agent<br/>Valid: 24 hours<br/>Signed by: Istio Root CA
    
    CA-->>ZT: Certificate + Private Key
    
    ZT->>ZT: Store certificate for<br/>SPIFFE: .../sa/pr-agent<br/>Associated with pod IP
    
    Note over ZT: Certificate ready<br/>Auto-rotates in 12 hours

    POD->>ZT: Application sends traffic<br/>(intercepted by TPROXY)
    
    ZT->>ZT: Use certificate for<br/>mTLS authentication<br/>Present SPIFFE ID

    Note over POD,CA: All traffic now authenticated<br/>and encrypted with mTLS
```

**Description:** Shows the complete certificate lifecycle from pod creation to mTLS usage, including CSR, issuance, storage, and automatic rotation.

---

## Diagram 10: SPIFFE ID-Based Authorization

```mermaid
graph TB
    START([Ztunnel receives<br/>connection request])
    
    EXTRACT[Extract Client SPIFFE ID<br/>from mTLS certificate]
    
    EXAMPLE[Example:<br/>spiffe://cluster.local/ns/agensys-demo-1/sa/pr-agent]
    
    PARSE[Parse SPIFFE ID components:<br/>Trust Domain: cluster.local<br/>Namespace: agensys-demo-1<br/>ServiceAccount: pr-agent]
    
    POLICY_LOOKUP[Look up Authorization Policy<br/>for destination service]
    
    CHECK_PRINCIPALS{Does SPIFFE ID match<br/>allowed principals in policy?}
    
    POLICY_EXAMPLE["Policy says:<br/>principals: ['cluster.local/ns/agensys-demo-1/sa/orchestrator-agent']<br/><br/>Client has:<br/>'cluster.local/ns/agensys-demo-1/sa/pr-agent'"]
    
    NO_MATCH[❌ SPIFFE IDs don't match]
    MATCH[✅ SPIFFE ID matches]
    
    DENY[Deny Connection<br/>Log: RBAC_ACCESS_DENIED<br/>Metric: blocked_connection++]
    
    ALLOW[Allow Connection<br/>Forward to destination<br/>Metric: allowed_connection++]
    
    END_DENY([Client receives timeout])
    END_ALLOW([Client receives response])

    START --> EXTRACT
    EXTRACT --> EXAMPLE
    EXAMPLE --> PARSE
    PARSE --> POLICY_LOOKUP
    POLICY_LOOKUP --> CHECK_PRINCIPALS
    CHECK_PRINCIPALS --> POLICY_EXAMPLE
    POLICY_EXAMPLE --> NO_MATCH
    POLICY_EXAMPLE --> MATCH
    NO_MATCH --> DENY
    MATCH --> ALLOW
    DENY --> END_DENY
    ALLOW --> END_ALLOW

    classDef success fill:#c8e6c9,stroke:#2e7d32,stroke-width:2px,color:#000
    classDef failure fill:#ffcdd2,stroke:#c62828,stroke-width:2px,color:#000
    classDef check fill:#fff9c4,stroke:#f57f17,stroke-width:2px,color:#000
    classDef info fill:#e1f5fe,stroke:#0277bd,stroke-width:2px,color:#000

    class MATCH,ALLOW,END_ALLOW success
    class NO_MATCH,DENY,END_DENY failure
    class CHECK_PRINCIPALS check
    class EXTRACT,PARSE,POLICY_LOOKUP,EXAMPLE,POLICY_EXAMPLE info
```

**Description:** Detailed flowchart showing how ztunnel extracts SPIFFE IDs from mTLS certificates and uses them for identity-based authorization decisions.

---

## Diagram 11: ServiceEntry + AuthorizationPolicy for External APIs

```mermaid
graph TB
    subgraph "Without ServiceEntry"
        APP1[PR Agent]
        ZT1[Ztunnel]
        DNS1[DNS Resolution]
        
        APP1 -->|curl api.openai.com| ZT1
        ZT1 -->|Unknown host| DNS1
        DNS1 -.->|❌ Service not in mesh<br/>Connection blocked| APP1
        
        NOTE1[Result: Connection fails<br/>External host unknown to mesh]
    end

    ARROW1[Add ServiceEntry]

    subgraph "With ServiceEntry Only"
        APP2[PR Agent]
        ZT2[Ztunnel]
        SE[ServiceEntry:<br/>api.openai.com:443]
        
        APP2 -->|curl api.openai.com| ZT2
        ZT2 -->|Check ServiceEntry| SE
        SE -.->|Host known, but...<br/>❌ No AuthZ policy<br/>Falls back to default-deny| APP2
        
        NOTE2[Result: Connection blocked<br/>Host known but not authorized]
    end

    ARROW2[Add AuthorizationPolicy]

    subgraph "With ServiceEntry + AuthorizationPolicy"
        APP3[PR Agent<br/>SPIFFE: .../sa/pr-agent]
        ZT3[Ztunnel]
        SE2[ServiceEntry:<br/>api.openai.com:443]
        AUTHZ[AuthorizationPolicy:<br/>ALLOW pr-agent<br/>→ api.openai.com:443]
        OPENAI[api.openai.com]
        
        APP3 -->|curl api.openai.com| ZT3
        ZT3 -->|1. Check ServiceEntry| SE2
        SE2 -->|✅ Host known| ZT3
        ZT3 -->|2. Check AuthZ| AUTHZ
        AUTHZ -->|✅ pr-agent allowed| ZT3
        ZT3 ==>|3. Establish mTLS| OPENAI
        OPENAI ==>|Response| APP3
        
        NOTE3[Result: Connection succeeds<br/>Host known AND authorized]
    end

    NOTE1 --> ARROW1
    ARROW1 --> NOTE2
    NOTE2 --> ARROW2
    ARROW2 --> NOTE3

    classDef blocked fill:#ffcdd2,stroke:#c62828,stroke-width:2px,color:#000
    classDef allowed fill:#c8e6c9,stroke:#2e7d32,stroke-width:2px,color:#000
    classDef config fill:#fff9c4,stroke:#f57f17,stroke-width:2px,color:#000

    class APP1,ZT1,DNS1,APP2,ZT2,NOTE1,NOTE2 blocked
    class APP3,ZT3,SE2,AUTHZ,OPENAI,NOTE3 allowed
    class SE config
```

**Description:** Shows why both ServiceEntry (makes host visible) and AuthorizationPolicy (grants access) are required for external API access.

---

## Diagram 12: Real-World Traffic Example - PR Agent to OpenAI

```mermaid
sequenceDiagram
    participant APP as PR Agent<br/>Application Code
    participant IPTABLES as iptables TPROXY<br/>(Pod netns)
    participant ZT_SRC as Ztunnel Node 1<br/>(Source)
    participant ZT_DST as Ztunnel Node 2<br/>(Destination/Egress)
    participant OPENAI as api.openai.com<br/>OpenAI API

    Note over APP: Python code executes:<br/>requests.post("https://api.openai.com/v1/chat")

    APP->>IPTABLES: TCP SYN to 104.18.7.192:443<br/>(DNS already resolved)
    
    Note over IPTABLES: iptables rule matches:<br/>TPROXY redirect to :15001
    
    IPTABLES->>ZT_SRC: Redirected connection<br/>Original dest: api.openai.com:443<br/>Source: PR Agent pod IP

    Note over ZT_SRC: 1. Extract pod identity<br/>ServiceAccount: pr-agent<br/>SPIFFE: .../sa/pr-agent

    Note over ZT_SRC: 2. Check ServiceEntry<br/>✅ api.openai.com exists

    Note over ZT_SRC: 3. Check AuthZ Policy<br/>allow-pr-agent-to-openai<br/>principals: [".../sa/pr-agent"]<br/>hosts: ["api.openai.com"]<br/>✅ MATCH

    ZT_SRC->>ZT_DST: HBONE HTTP/2 CONNECT<br/>+ mTLS (node-to-node)
    
    Note over ZT_SRC,ZT_DST: mTLS Tunnel Established<br/>Certificate exchange<br/>Both nodes authenticated

    ZT_DST->>OPENAI: TLS Handshake<br/>to api.openai.com:443

    OPENAI-->>ZT_DST: TLS ServerHello + Certificate

    Note over ZT_DST: Verify OpenAI certificate<br/>against public CAs

    ZT_DST->>OPENAI: HTTP POST /v1/chat<br/>Authorization: Bearer sk-...

    OPENAI-->>ZT_DST: HTTP 200 OK<br/>JSON response with completion

    ZT_DST-->>ZT_SRC: Response via HBONE tunnel

    ZT_SRC-->>IPTABLES: Response

    IPTABLES-->>APP: Response delivered<br/>to application

    Note over APP: Application receives:<br/>response.status_code = 200<br/>response.json() = {...}

    Note over APP,OPENAI: Entire flow transparent<br/>to application code
```

**Description:** Real-world example showing complete traffic flow from application code through TPROXY, ztunnel, HBONE tunnel, policy checks, to external API and back.

---

## Diagram 13: Policy Violation Example - PR Agent to Anthropic (Blocked)

```mermaid
sequenceDiagram
    participant APP as PR Agent<br/>Application Code
    participant IPTABLES as iptables TPROXY
    participant ZT as Ztunnel Node 1
    participant LOG as Ztunnel Logs
    participant METRICS as Prometheus Metrics

    Note over APP: Application tries:<br/>requests.post("https://api.anthropic.com/v1/messages")

    APP->>IPTABLES: TCP SYN to api.anthropic.com:443

    IPTABLES->>ZT: TPROXY redirect

    Note over ZT: Extract identity:<br/>SPIFFE: .../sa/pr-agent

    Note over ZT: Check ServiceEntry:<br/>Looking for anthropic-api...

    alt ServiceEntry exists
        Note over ZT: ✅ ServiceEntry found<br/>Host: api.anthropic.com
        
        Note over ZT: Check AuthZ Policy:<br/>allow-summary-agent-to-anthropic
        
        Note over ZT: Policy requires:<br/>principals: [".../sa/summary-agent"]<br/><br/>Request has:<br/>principal: ".../sa/pr-agent"<br/><br/>❌ NO MATCH
    else ServiceEntry doesn't exist
        Note over ZT: ❌ No ServiceEntry<br/>Host unknown to mesh
    end

    ZT->>LOG: Log entry:<br/>[ERROR] RBAC_ACCESS_DENIED<br/>source: pr-agent<br/>destination: api.anthropic.com<br/>reason: principal_mismatch

    ZT->>METRICS: Increment counter:<br/>istio_tcp_connections_closed_total{<br/>  response_flags="RBAC_ACCESS_DENIED",<br/>  source_workload="pr-agent",<br/>  destination_host="api.anthropic.com"<br/>}

    Note over ZT: Drop connection<br/>No response sent

    ZT-->>IPTABLES: Connection timeout (no response)

    IPTABLES-->>APP: Connection timeout

    Note over APP: Application receives:<br/>requests.exceptions.Timeout<br/>After 10 seconds<br/>(or configured timeout)

    Note over APP: Application must handle:<br/>- Retry logic?<br/>- Error logging?<br/>- Fallback behavior?
```

**Description:** Shows what happens when a policy violation occurs - the connection is silently dropped, logged, and monitored, resulting in a timeout for the application.


---
