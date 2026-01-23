# AgentGateway Manifests

This directory contains Kubernetes manifests for deploying AgentGateway with Istio Ambient Mode for the autonomous code review system described in the blog post series.

## Quick Start

```bash
# 1. Create namespace
kubectl apply -f 01-namespace.yaml

# 2. Create secrets (EDIT THIS FILE FIRST with your actual API keys!)
kubectl apply -f 02-secrets.yaml

# 3. Deploy AgentGateway configuration
kubectl apply -f 03-agentgateway-config.yaml

# 4. Deploy AgentGateway
kubectl apply -f 04-agentgateway-deployment.yaml

# 5. Create service
kubectl apply -f 05-agentgateway-service.yaml

# 6. (Optional) Deploy MCP servers if you have them
# kubectl apply -f 06-mcp-servers.yaml

# 7. (Optional) Apply Istio integration if using Istio Ambient Mode
# kubectl apply -f 07-istio-integration.yaml
```

## File Overview

| File | Description | Required |
|------|-------------|----------|
| `01-namespace.yaml` | Creates the namespace | Yes |
| `02-secrets.yaml` | API keys for Anthropic, OpenAI, GitHub | Yes |
| `03-agentgateway-config.yaml` | AgentGateway configuration | Yes |
| `04-agentgateway-deployment.yaml` | AgentGateway deployment | Yes |
| `05-agentgateway-service.yaml` | Service exposing gateway ports | Yes |
| `06-mcp-servers.yaml` | GitHub and Semgrep MCP servers | Optional |
| `07-istio-integration.yaml` | Istio ServiceEntry and AuthorizationPolicy | Optional |

## Important: Update Secrets

Before deploying, **edit `02-secrets.yaml`** and replace the placeholder values with your actual API keys:

```yaml
stringData:
  ANTHROPIC_API_KEY: "sk-ant-api03-YOUR_ACTUAL_KEY_HERE"
  OPENAI_API_KEY: "sk-proj-YOUR_ACTUAL_KEY_HERE"
  GITHUB_TOKEN: "ghp_YOUR_ACTUAL_TOKEN_HERE"
```

## Verify Deployment

```bash
# Check if pods are running
kubectl get pods -n agensys-codereview-demo

# Check if service is ready
kubectl get svc agentgateway -n agensys-codereview-demo

# View logs
kubectl logs -n agensys-codereview-demo -l app.kubernetes.io/component=agentgateway

# Port-forward to test locally
kubectl port-forward -n agensys-codereview-demo svc/agentgateway 9080:9080
```

## Testing the Gateway

Test the health endpoint:
```bash
curl http://localhost:9080/healthz
```

Test Anthropic routing (requires port-forward):
```bash
curl -X POST http://localhost:9081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 100
  }'
```

## Port Reference

| Port | Purpose |
|------|---------|
| 9080 | Main orchestrator proxy |
| 9081 | Anthropic AI proxy |
| 9082 | OpenAI proxy |
| 9083 | MCP server proxy (if configured) |
| 15000 | Admin UI |
| 15020 | Prometheus metrics |

## MCP Server Configuration

If you want to use MCP servers (GitHub, Semgrep), you need to:

1. Deploy the MCP servers: `kubectl apply -f 06-mcp-servers.yaml`
2. Add port 9083 configuration to `03-agentgateway-config.yaml` (see blog post for details)
3. Update the AgentGateway deployment: `kubectl rollout restart deployment/agentgateway -n agensys-codereview-demo`

## Istio Integration

If using Istio Ambient Mode:

1. Ensure Istio is installed with ambient mode enabled
2. Apply the Istio integration manifests: `kubectl apply -f 07-istio-integration.yaml`

The ServiceEntry resources allow traffic to external LLM providers, and the AuthorizationPolicy restricts access to the gateway.

## Troubleshooting

**Pod not starting:**
```bash
kubectl describe pod -n agensys-codereview-demo -l app.kubernetes.io/component=agentgateway
```

**Secrets not loading:**
```bash
kubectl get secret api-secrets -n agensys-codereview-demo -o yaml
```

**Gateway returning 403:**
- Check if API keys are correctly set in the secret
- Verify the secret is mounted: `kubectl exec -n agensys-codereview-demo deployment/agentgateway -- env | grep API_KEY`

**Connection refused:**
- Verify service endpoints: `kubectl get endpoints agentgateway -n agensys-codereview-demo`
- Check if pods are ready: `kubectl get pods -n agensys-codereview-demo`

## Resources

- Blog Post: [Agents, MCP, and Kubernetes, Part 3](https://prashantr30.github.io/t0-blog/agents-mcp-on-k8s-pt3/)
- AgentGateway Documentation: https://github.com/agentgateway/agentgateway
- Repository: https://github.com/mirantis/t0-blog

## License

These manifests are provided as examples for educational purposes based on the blog post series.
