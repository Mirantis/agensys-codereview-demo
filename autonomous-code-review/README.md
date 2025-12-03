# PR Review System - Kubernetes Deployment

## Quick Start

### 1. Create Namespace
```bash
kubectl apply -f namespace.yaml
```

### 2. Create Secrets

Create a file called `secrets.env` with your actual keys:

```bash
# secrets.env
SEMGREP_APP_TOKEN=your_semgrep_token_here
GITHUB_TOKEN=your_github_token_here
ANTHROPIC_API_KEY=your_anthropic_api_key_here
# Add any other secrets from your .env file
```

Create the Kubernetes secret:

```bash
kubectl create secret generic pr-review-secrets \
  --from-env-file=secrets.env \
  --namespace=pr-review
```

**Important**: Delete `secrets.env` after creating the secret:
```bash
rm secrets.env
```

### 3. Deploy Application

```bash
kubectl apply -f configmap.yaml
kubectl apply -f pvc.yaml
kubectl apply -f services.yaml
kubectl apply -f deployment-orchestrator.yaml
kubectl apply -f deployment-pr-agent.yaml
kubectl apply -f deployment-summarizer.yaml
kubectl apply -f deployment-github-mcp.yaml
```

### 4. Verify Deployment

```bash
kubectl get pods -n pr-review
kubectl get services -n pr-review
```

### 5. Access the Orchestrator

Get the external IP:
```bash
kubectl get service orchestrator -n pr-review
```

The orchestrator will be available at `http://<EXTERNAL-IP>:8085`

## Notes

- The orchestrator service is exposed as LoadBalancer (change to NodePort or Ingress if needed)
- Requires a StorageClass that supports ReadWriteMany for the shared work volume
- Update image pull policy if using a private registry
- For production, consider adding resource limits, health checks, and proper ingress configuration
