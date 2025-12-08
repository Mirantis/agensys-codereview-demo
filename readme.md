# AI PR Review System - Kubernetes Deployment Guide

A complete AI-powered pull request review system with automated security scanning, code analysis, and intelligent feedback generation.

## 📋 Table of Contents

1. [Prerequisites](#prerequisites)
2. [Architecture Overview](#architecture-overview)
3. [Installation Steps](#installation-steps)
4. [Configuration](#configuration)
5. [Deployment](#deployment)
6. [Verification](#verification)
7. [GitHub Webhook Setup](#github-webhook-setup)
8. [Troubleshooting](#troubleshooting)

## 🎯 Prerequisites

### 1. Kubernetes Cluster

You need a running Kubernetes cluster. We recommend **k0s** for simplicity.

#### Install k0s

Follow the official k0s installation guide:
**📚 [k0s Installation Documentation](https://docs.k0sproject.io/stable/install/)**

Quick install on Ubuntu/Debian:
```bash
curl -sSLf https://get.k0s.sh | sudo sh
sudo k0s install controller --single
sudo k0s start
sudo k0s status
```

Configure kubectl:
```bash
# Get kubeconfig
sudo k0s kubeconfig admin > ~/.kube/config

# Verify cluster
kubectl get nodes
kubectl get pods -A
```

### 2. Required API Keys

You'll need API keys for:
- **OpenAI** - For PR-Agent (GPT-4)
- **Anthropic** - For Summarizer (Claude)
- **GitHub** - Personal Access Token with repo permissions
- **Semgrep** - App Token from semgrep.com (optional but recommended)

### 3. Docker Images

Ensure all images are built and pushed to your registry (e.g., ghcr.io):
- `ghcr.io/YOUR_USERNAME/orchestrator:VERSION`
- `ghcr.io/YOUR_USERNAME/pr-agent:VERSION`
- `ghcr.io/YOUR_USERNAME/summarizer-agent:VERSION`
- `ghcr.io/YOUR_USERNAME/github-mcp-server:VERSION`
- `ghcr.io/YOUR_USERNAME/semgrep-service:VERSION`

## 🏗️ Architecture Overview

![Architecture Diagram](./assets/architecture.png)

**Services:**
- **Orchestrator** - Main coordinator, receives GitHub webhooks
- **PR-Agent** - Generates PR descriptions and reviews (GPT-4)
- **Semgrep Service** - Security scanning and vulnerability detection
- **Summarizer Agent** - Synthesizes all feedback (Claude)
- **GitHub MCP Server** - Posts comments back to GitHub

## 🚀 Installation Steps

### Step 1: Create Namespace

```bash
kubectl apply -f ns.yml
```

Verify:
```bash
kubectl get namespace agensys-demo-1
```

### Step 2: Create API Secrets

Create a file `keys_tokens.txt` with your API keys:

```bash
cat > keys_tokens.txt << 'EOF'
OPENAI_API_KEY=sk-proj-your-openai-key-here
ANTHROPIC_API_KEY=sk-ant-api03-your-anthropic-key-here
GITHUB_TOKEN=ghp_your-github-token-here
SEMGREP_APP_TOKEN=your-semgrep-token-here
EOF
```

Create the secret:
```bash
kubectl create secret generic api-secrets \
  --from-env-file=keys_tokens.txt \
  -n agensys-demo-1

# Verify
kubectl get secret api-secrets -n agensys-demo-1
```

**⚠️ Important:** Delete `keys_tokens.txt` after creating the secret:
```bash
rm keys_tokens.txt
```

### Step 3: Create Docker Registry Secret

If using a private registry (e.g., GitHub Container Registry):

```bash
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=YOUR_GITHUB_USERNAME \
  --docker-password=YOUR_GITHUB_PAT \
  --docker-email=YOUR_EMAIL \
  --namespace=agensys-demo-1

# Verify
kubectl get secret ghcr-secret -n agensys-demo-1
```

### Step 4: Deploy Services

Deploy in the following order:

#### 4.1 Deploy Semgrep Service

```bash
# Deploy
kubectl apply -f semgrep/deploy.yml
kubectl apply -f semgrep/svc.yml

# Wait for ready
kubectl wait --for=condition=available --timeout=60s \
  deployment/semgrep-service -n agensys-demo-1

# Verify
kubectl get pods -n agensys-demo-1 -l app=semgrep-service
kubectl logs -l app=semgrep-service -n agensys-demo-1 --tail=20
```

#### 4.2 Deploy PR-Agent

```bash
# Deploy
kubectl apply -f pr-agent/cm.yml
kubectl apply -f pr-agent/deploy.yml
kubectl apply -f pr-agent/svc.yml

# Wait for ready
kubectl wait --for=condition=available --timeout=60s \
  deployment/pr-agent -n agensys-demo-1

# Verify
kubectl get pods -n agensys-demo-1 -l app=pr-agent
kubectl logs -l app=pr-agent -n agensys-demo-1 --tail=20
```

#### 4.3 Deploy Summarizer Agent

```bash
# Deploy
kubectl apply -f summarizer-agent/cm.yml
kubectl apply -f summarizer-agent/deploy.yml
kubectl apply -f summarizer-agent/svc.yml

# Wait for ready
kubectl wait --for=condition=available --timeout=60s \
  deployment/summarizer-agent -n agensys-demo-1

# Verify
kubectl get pods -n agensys-demo-1 -l app=summarizer-agent
kubectl logs -l app=summarizer-agent -n agensys-demo-1 --tail=20
```

#### 4.4 Deploy GitHub MCP Server

```bash
# Deploy
kubectl apply -f github-mcp/cm.yml
kubectl apply -f github-mcp/deploy.yml
kubectl apply -f github-mcp/svc.yml

# Wait for ready
kubectl wait --for=condition=available --timeout=60s \
  deployment/github-mcp-server -n agensys-demo-1

# Verify
kubectl get pods -n agensys-demo-1 -l app=github-mcp-server
kubectl logs -l app=github-mcp-server -n agensys-demo-1 --tail=20
```

#### 4.5 Deploy Orchestrator (Last)

```bash
# Deploy
kubectl apply -f orchestrator/cm.yml
kubectl apply -f orchestrator/deploy.yml
kubectl apply -f orchestrator/svc.yml

# Wait for ready
kubectl wait --for=condition=available --timeout=60s \
  deployment/orchestrator -n agensys-demo-1

# Verify
kubectl get pods -n agensys-demo-1 -l app=orchestrator
kubectl logs -l app=orchestrator -n agensys-demo-1 --tail=20
```

#### 4.6 Quick Deploy Script

Alternatively, use the provided script:

```bash
# Make executable
chmod +x s.sh

# Deploy all
./s.sh

# Check status
kubectl get pods -n agensys-demo-1
```

## ✅ Verification

### Check All Services

```bash
# Check all pods are running
kubectl get pods -n agensys-demo-1

# Expected output:
# NAME                                READY   STATUS    RESTARTS   AGE
# orchestrator-xxx-xxx                1/1     Running   0          2m
# pr-agent-xxx-xxx                    1/1     Running   0          3m
# semgrep-service-xxx-xxx             1/1     Running   0          5m
# summarizer-agent-xxx-xxx            1/1     Running   0          3m
# github-mcp-server-xxx-xxx           1/1     Running   0          3m
```

### Check Services

```bash
kubectl get svc -n agensys-demo-1

# Expected output:
# NAME                TYPE        CLUSTER-IP       PORT(S)
# orchestrator        ClusterIP   10.x.x.x         8085/TCP
# pr-agent            ClusterIP   10.x.x.x         80/TCP
# semgrep-service     ClusterIP   10.x.x.x         80/TCP
# summarizer-agent    ClusterIP   10.x.x.x         80/TCP
# github-mcp-server   ClusterIP   10.x.x.x         80/TCP
```

### Test Internal Connectivity

```bash
# Get a shell in orchestrator
kubectl exec -it deployment/orchestrator -n agensys-demo-1 -- sh

# Inside the pod, test connectivity
wget -O- http://pr-agent/health 2>/dev/null || echo "Check logs"
wget -O- http://semgrep-service/health 2>/dev/null || echo "Check logs"
wget -O- http://summarizer-agent/health 2>/dev/null || echo "Check logs"
wget -O- http://github-mcp-server/health 2>/dev/null || echo "Check logs"
exit
```

## 🔗 GitHub Webhook Setup

### Step 1: Expose Orchestrator

Choose one option:

#### Option A: NodePort (Quick test)

```bash
kubectl patch svc orchestrator -n agensys-demo-1 \
  -p '{"spec":{"type":"NodePort"}}'

# Get the node port
kubectl get svc orchestrator -n agensys-demo-1

# Your webhook URL: http://NODE_IP:NODE_PORT/webhook
```

#### Option B: LoadBalancer (Cloud)

```bash
kubectl patch svc orchestrator -n agensys-demo-1 \
  -p '{"spec":{"type":"LoadBalancer"}}'

# Wait for external IP
kubectl get svc orchestrator -n agensys-demo-1 -w

# Your webhook URL: http://EXTERNAL_IP:8085/webhook
```

#### Option C: Ingress (Production)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: orchestrator-ingress
  namespace: agensys-demo-1
spec:
  rules:
  - host: ai-pr-review.yourdomain.com
    http:
      paths:
      - path: /webhook
        pathType: Prefix
        backend:
          service:
            name: orchestrator
            port:
              number: 8085
```

### Step 2: Configure GitHub Repository

1. Go to your GitHub repository
2. Click **Settings** → **Webhooks** → **Add webhook**
3. Configure:
   - **Payload URL**: `http://YOUR_IP:PORT/webhook`
   - **Content type**: `application/json`
   - **Events**: Select "Pull requests"
   - **Active**: ✅ Checked

4. Click **Add webhook**

### Step 3: Test Webhook

1. Create a new PR in your repository
2. Check orchestrator logs:

```bash
kubectl logs -f deployment/orchestrator -n agensys-demo-1
```

You should see:
```json
{"level":"info","message":"received GitHub PR webhook"}
{"level":"info","message":"starting PR processing pipeline"}
```

## 📊 Monitoring

### Watch Logs in Real-Time

```bash
# All orchestrator logs
kubectl logs -f deployment/orchestrator -n agensys-demo-1

# Specific service
kubectl logs -f deployment/pr-agent -n agensys-demo-1
kubectl logs -f deployment/semgrep-service -n agensys-demo-1
kubectl logs -f deployment/summarizer-agent -n agensys-demo-1
kubectl logs -f deployment/github-mcp-server -n agensys-demo-1

# All pods with label
kubectl logs -f -l app=orchestrator -n agensys-demo-1
```

### Check Resource Usage

```bash
kubectl top pods -n agensys-demo-1
```

### Debug a Specific PR Processing

```bash
# Follow orchestrator logs and trigger a PR
kubectl logs -f deployment/orchestrator -n agensys-demo-1 | grep "pr_number"
```

## 🐛 Troubleshooting

### Issue 1: Pods Not Starting

```bash
# Check pod status
kubectl get pods -n agensys-demo-1

# Describe pod to see events
kubectl describe pod POD_NAME -n agensys-demo-1

# Check logs
kubectl logs POD_NAME -n agensys-demo-1
```

**Common causes:**
- Image pull errors → Check `ghcr-secret`
- Missing secrets → Verify `api-secrets` exists
- Resource constraints → Check `kubectl describe node`

### Issue 2: ImagePullBackOff

```bash
# Check image pull secret
kubectl get secret ghcr-secret -n agensys-demo-1

# Verify image exists
docker pull ghcr.io/YOUR_USERNAME/IMAGE:TAG

# Recreate secret if needed
kubectl delete secret ghcr-secret -n agensys-demo-1
# Then create again (Step 3)
```

### Issue 3: Service Returns 503

```bash
# Check if pod is ready
kubectl get pods -n agensys-demo-1

# Check service endpoints
kubectl get endpoints SERVICE_NAME -n agensys-demo-1

# Test from orchestrator pod
kubectl exec -it deployment/orchestrator -n agensys-demo-1 -- \
  wget -O- http://SERVICE_NAME/health
```

### Issue 4: Orchestrator Not Processing PRs

```bash
# Check orchestrator logs
kubectl logs -f deployment/orchestrator -n agensys-demo-1

# Check if webhook is configured correctly
# Verify GitHub webhook deliveries in repo settings

# Test manually
curl -X POST http://ORCHESTRATOR_URL/webhook \
  -H "Content-Type: application/json" \
  -d '{"action":"opened","number":1,"pull_request":{...}}'
```

### Issue 5: Missing API Keys

```bash
# Check if secret exists
kubectl get secret api-secrets -n agensys-demo-1

# View secret keys (not values)
kubectl describe secret api-secrets -n agensys-demo-1

# Recreate if needed
kubectl delete secret api-secrets -n agensys-demo-1
# Then create again (Step 2)
```

## 🔄 Updates and Maintenance

### Update Image

```bash
# Build and push new image
docker build -t ghcr.io/YOUR_USERNAME/SERVICE:NEW_TAG .
docker push ghcr.io/YOUR_USERNAME/SERVICE:NEW_TAG

# Update deployment
kubectl set image deployment/SERVICE_NAME \
  CONTAINER_NAME=ghcr.io/YOUR_USERNAME/SERVICE:NEW_TAG \
  -n agensys-demo-1

# Watch rollout
kubectl rollout status deployment/SERVICE_NAME -n agensys-demo-1
```

### Rollback Deployment

```bash
# View history
kubectl rollout history deployment/SERVICE_NAME -n agensys-demo-1

# Rollback to previous
kubectl rollout undo deployment/SERVICE_NAME -n agensys-demo-1
```

### Scale Services

```bash
# Scale up
kubectl scale deployment/SERVICE_NAME --replicas=3 -n agensys-demo-1

# Scale down
kubectl scale deployment/SERVICE_NAME --replicas=1 -n agensys-demo-1
```

## 🗑️ Cleanup

### Remove All Resources

```bash
# Delete all deployments
kubectl delete -f orchestrator/
kubectl delete -f pr-agent/
kubectl delete -f semgrep/
kubectl delete -f summarizer-agent/
kubectl delete -f github-mcp/

# Delete secrets
kubectl delete secret api-secrets -n agensys-demo-1
kubectl delete secret ghcr-secret -n agensys-demo-1

# Delete namespace (removes everything)
kubectl delete namespace agensys-demo-1
```

## 📚 Additional Resources

- **k0s Documentation**: https://docs.k0sproject.io/
- **Kubernetes Documentation**: https://kubernetes.io/docs/
- **GitHub Webhooks**: https://docs.github.com/en/webhooks
- **Semgrep**: https://semgrep.dev/
- **OpenAI API**: https://platform.openai.com/docs/
- **Anthropic API**: https://docs.anthropic.com/

## 🎉 Success Criteria

Your deployment is successful when:

1. ✅ All 5 pods are Running
2. ✅ All services have endpoints
3. ✅ Orchestrator logs show "starting orchestrator"
4. ✅ GitHub webhook deliveries show success (200)
5. ✅ PR comments appear automatically on new PRs

**Test with a real PR:**
1. Create a PR with some code
2. Check orchestrator logs: `kubectl logs -f deployment/orchestrator -n agensys-demo-1`
3. Wait 30-60 seconds
4. See automated review comment on GitHub PR! 🎊

---

**Need help?** Check the logs first, then review the troubleshooting section above.
