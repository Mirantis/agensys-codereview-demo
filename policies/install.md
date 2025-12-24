# Installing Istio Ambient Mode

This guide provides step-by-step instructions for installing Istio with Ambient Mode support and verifying the installation.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
  - [Method 1: Helm (Recommended)](#method-1-helm-recommended-for-production)
  - [Method 2: istioctl](#method-2-istioctl)
- [Enable Ambient Mode for Namespace](#enable-ambient-mode-for-namespace)
- [Verification Tests](#verification-tests)
- [Deploy Test Application](#deploy-test-application)
- [Test Ambient Functionality](#test-ambient-functionality)
- [Health Check Script](#health-check-script)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before installing Istio Ambient Mode, ensure you have:

### System Requirements

- **Kubernetes cluster**: Version 1.27 or higher (1.29+ recommended)
- **kubectl**: Configured and connected to your cluster
- **Minimum resources per node**:
  - 2 CPUs
  - 4GB RAM
  - 20GB disk space

### Verify Prerequisites

```bash
# Check Kubernetes version
kubectl version --short

# Check cluster connectivity
kubectl cluster-info

# Check available resources
kubectl top nodes
```

### Install Required Tools

#### Install Helm (if not already installed)

```bash
# macOS
brew install helm

# Linux
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Verify
helm version
```

#### Install istioctl (if using Method 2)

```bash
# Download latest Istio
curl -L https://istio.io/downloadIstio | sh -

# Move to Istio directory
cd istio-1.24.0  # or your version

# Add istioctl to PATH
export PATH=$PWD/bin:$PATH

# Verify
istioctl version
```

---

## Installation Methods

Choose one of the following methods:

---

## Method 1: Helm (Recommended for Production)

### Step 1: Add Istio Helm Repository

```bash
# Add Istio Helm repository
helm repo add istio https://istio-release.storage.googleapis.com/charts

# Update Helm repositories
helm repo update

# Verify repository
helm search repo istio
```

### Step 2: Create Istio System Namespace

```bash
# Create namespace for Istio components
kubectl create namespace istio-system
```

### Step 3: Install Istio Base (CRDs)

```bash
# Install Istio base components (Custom Resource Definitions)
helm install istio-base istio/base \
  -n istio-system \
  --wait
```

**Verify CRDs:**
```bash
kubectl get crd | grep istio.io
```

**Expected output** (should see multiple CRDs):
```
authorizationpolicies.security.istio.io
destinationrules.networking.istio.io
envoyfilters.networking.istio.io
gateways.networking.istio.io
peerauthentications.security.istio.io
proxyconfigs.networking.istio.io
requestauthentications.security.istio.io
serviceentries.networking.istio.io
sidecars.networking.istio.io
telemetries.telemetry.istio.io
virtualservices.networking.istio.io
wasmplugins.extensions.istio.io
workloadentries.networking.istio.io
workloadgroups.networking.istio.io
```

### Step 4: Install Istiod (Control Plane)

```bash
# Install istiod control plane
helm install istiod istio/istiod \
  -n istio-system \
  --wait
```

**Verify istiod:**
```bash
kubectl get pods -n istio-system -l app=istiod
```

**Expected output:**
```
NAME                      READY   STATUS    RESTARTS   AGE
istiod-5c6c9c5d8f-xxxxx   1/1     Running   0          1m
```

**Check istiod logs:**
```bash
kubectl logs -n istio-system -l app=istiod --tail=20
```

Should see: "initialization complete" and no errors.

### Step 5: Install Istio CNI (Required for Ambient Mode)

```bash
# Install Istio CNI plugin with ambient profile
helm install istio-cni istio/cni \
  -n istio-system \
  --set profile=ambient \
  --wait
```

**Verify CNI:**
```bash
kubectl get pods -n istio-system -l app=istio-cni-node
```

**Expected output** (one pod per node):
```
NAME                      READY   STATUS    RESTARTS   AGE
istio-cni-node-xxxxx      1/1     Running   0          1m
istio-cni-node-yyyyy      1/1     Running   0          1m
istio-cni-node-zzzzz      1/1     Running   0          1m
```

**Check CNI logs:**
```bash
kubectl logs -n istio-system -l app=istio-cni-node --tail=20
```

Should see: "CNI installation complete" or similar success messages.

### Step 6: Install Ztunnel (Ambient Data Plane)

```bash
# Install ztunnel DaemonSet (the ambient data plane)
helm install ztunnel istio/ztunnel \
  -n istio-system \
  --wait
```

**Verify ztunnel:**
```bash
kubectl get pods -n istio-system -l app=ztunnel
```

**Expected output** (one pod per node):
```
NAME               READY   STATUS    RESTARTS   AGE
ztunnel-xxxxx      1/1     Running   0          1m
ztunnel-yyyyy      1/1     Running   0          1m
ztunnel-zzzzz      1/1     Running   0          1m
```

**Verify DaemonSet:**
```bash
kubectl get daemonset -n istio-system ztunnel
```

**Expected:**
```
NAME      DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
ztunnel   3         3         3       3            3           <none>          1m
```

### Step 7: Verify Complete Installation

```bash
# Check all Istio components
kubectl get pods -n istio-system
```

**Expected output:**
```
NAME                                   READY   STATUS    RESTARTS   AGE
istio-cni-node-xxxxx                   1/1     Running   0          2m
istio-cni-node-yyyyy                   1/1     Running   0          2m
istio-cni-node-zzzzz                   1/1     Running   0          2m
istiod-5c6c9c5d8f-xxxxx                1/1     Running   0          2m
ztunnel-xxxxx                          1/1     Running   0          1m
ztunnel-yyyyy                          1/1     Running   0          1m
ztunnel-zzzzz                          1/1     Running   0          1m
```

---

## Method 2: istioctl

### Step 1: Download and Install istioctl

```bash
# Download latest Istio release
curl -L https://istio.io/downloadIstio | sh -

# Navigate to Istio directory
cd istio-1.24.0  # Replace with your version

# Add istioctl to PATH
export PATH=$PWD/bin:$PATH

# Verify installation
istioctl version
```

### Step 2: Install Istio with Ambient Profile

```bash
# Install using the ambient profile
istioctl install --set profile=ambient -y
```

This single command installs:
- Istio base (CRDs)
- Istiod (control plane)
- Istio CNI
- Ztunnel (ambient data plane)

**Expected output:**
```
✔ Istio core installed
✔ Istiod installed
✔ CNI installed
✔ Ztunnel installed
✔ Installation complete
```

### Step 3: Verify Installation

```bash
# Check all components
kubectl get pods -n istio-system
```

**Expected output:**
```
NAME                                   READY   STATUS    RESTARTS   AGE
istio-cni-node-xxxxx                   1/1     Running   0          2m
istio-cni-node-yyyyy                   1/1     Running   0          2m
istiod-5c6c9c5d8f-xxxxx                1/1     Running   0          2m
ztunnel-xxxxx                          1/1     Running   0          2m
ztunnel-yyyyy                          1/1     Running   0          2m
```

### Step 4: Check Version Consistency

```bash
# Verify all components have matching versions
istioctl version
```

**Expected output:**
```
client version: 1.24.0
control plane version: 1.24.0
data plane version: 1.24.0 (ztunnel)
```

---

## Enable Ambient Mode for Namespace

After installing Istio, you need to enable ambient mode for your application namespace.

### Step 1: Create Your Namespace

```bash
# Create namespace for your application
kubectl create namespace agensys-demo-1
```

### Step 2: Enable Ambient Mode

```bash
# Label namespace to enable ambient data plane
kubectl label namespace agensys-demo-1 istio.io/dataplane-mode=ambient
```

### Step 3: Verify Namespace Configuration

```bash
# Check namespace label
kubectl get namespace agensys-demo-1 -o jsonpath='{.metadata.labels.istio\.io/dataplane-mode}'
```

**Expected output:**
```
ambient
```

**View full namespace details:**
```bash
kubectl get namespace agensys-demo-1 -o yaml
```

Should contain:
```yaml
metadata:
  labels:
    istio.io/dataplane-mode: ambient
  name: agensys-demo-1
```

---

## Verification Tests

### Test 1: Verify Istio Version

```bash
istioctl version
```

**Expected output:**
```
client version: 1.24.0
control plane version: 1.24.0
data plane version: 1.24.0 (ztunnel)
```

All versions should match.

### Test 2: Verify All Components Are Running

```bash
kubectl get pods -n istio-system
```

**Required components:**
- ✅ **istiod**: 1 pod, Running
- ✅ **istio-cni-node**: 1 per node, Running
- ✅ **ztunnel**: 1 per node, Running

### Test 3: Check Istiod Health

```bash
# Check istiod is responding
kubectl exec -n istio-system deploy/istiod -- curl -s localhost:15014/ready
```

**Expected:** HTTP 200 response (empty body is OK)

**Check istiod logs for errors:**
```bash
kubectl logs -n istio-system -l app=istiod --tail=50 | grep -i error
```

**Expected:** No error messages (empty output)

### Test 4: Verify Ztunnel DaemonSet

```bash
# Check ztunnel DaemonSet status
kubectl get daemonset -n istio-system ztunnel
```

**Expected:**
```
NAME      DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
ztunnel   3         3         3       3            3           <none>          5m
```

- DESIRED = number of nodes in cluster
- CURRENT = DESIRED
- READY = DESIRED

**Check ztunnel is healthy:**
```bash
kubectl logs -n istio-system -l app=ztunnel --tail=20
```

Should see normal operation logs, no errors.

### Test 5: Verify CNI Installation

```bash
# Check CNI DaemonSet
kubectl get daemonset -n istio-system istio-cni-node
```

**Expected:**
```
NAME              DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   AGE
istio-cni-node    3         3         3       3            3           5m
```

**Check CNI logs:**
```bash
kubectl logs -n istio-system -l app=istio-cni-node --tail=20
```

Should see: "CNI installation complete" or similar success messages.

### Test 6: List Ambient-Enabled Namespaces

```bash
# List all namespaces with ambient mode enabled
kubectl get namespaces -l istio.io/dataplane-mode=ambient
```

**Expected:**
```
NAME              STATUS   AGE
agensys-demo-1    Active   5m
```

### Test 7: Verify Istio CRDs

```bash
# Check Istio Custom Resource Definitions exist
kubectl get crd | grep istio.io | wc -l
```

**Expected:** Should return a number (typically 14+ CRDs)

**List all Istio CRDs:**
```bash
kubectl get crd | grep istio.io
```

---

## Deploy Test Application

Deploy a simple test application to verify ambient mode is working correctly.

### Step 1: Deploy Test Server

```bash
# Deploy nginx as test server
kubectl create deployment test-server --image=nginx:latest -n agensys-demo-1

# Expose as service
kubectl expose deployment test-server --port=80 --name=test-server -n agensys-demo-1

# Wait for pod to be ready
kubectl wait --for=condition=ready pod -l app=test-server -n agensys-demo-1 --timeout=60s
```

### Step 2: Deploy Test Client

```bash
# Deploy curl as test client
kubectl create deployment test-client \
  --image=curlimages/curl:latest \
  -n agensys-demo-1 \
  -- sleep infinity

# Wait for pod to be ready
kubectl wait --for=condition=ready pod -l app=test-client -n agensys-demo-1 --timeout=60s
```

### Step 3: Verify No Sidecars Injected

**This is critical - Ambient mode should NOT inject sidecars.**

```bash
# Check test-server containers
kubectl get pod -n agensys-demo-1 -l app=test-server -o jsonpath='{.items[0].spec.containers[*].name}'
```

**Expected output:**
```
nginx
```

Should show ONLY `nginx` (NO `istio-proxy` sidecar).

**Check test-client containers:**
```bash
kubectl get pod -n agensys-demo-1 -l app=test-client -o jsonpath='{.items[0].spec.containers[*].name}'
```

**Expected output:**
```
curl
```

Should show ONLY `curl` (NO `istio-proxy` sidecar).

✅ **If you see only the application container (no istio-proxy), ambient mode is working correctly!**

---

## Test Ambient Functionality

### Test 1: Basic Connectivity

```bash
# Test client can reach server
kubectl exec -n agensys-demo-1 deploy/test-client -- \
  curl -s http://test-server
```

**Expected output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
...
```

✅ Connection should succeed and return nginx welcome page.

### Test 2: Verify mTLS Encryption

**Check ztunnel logs for mTLS connections:**
```bash
kubectl logs -n istio-system -l app=ztunnel --tail=100 | grep -E "(test-server|test-client)"
```

**Expected:** Should see log entries showing mTLS connections between test-client and test-server.

**Check ztunnel metrics:**
```bash
kubectl exec -n istio-system -l app=ztunnel -c ztunnel -- \
  curl -s localhost:15020/stats/prometheus | grep istio_tcp_connections
```

Should show TCP connection metrics with TLS.

### Test 3: Test Authorization Policies

**Deploy a deny-all policy:**
```bash
cat <<EOF | kubectl apply -f -
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: test-deny-all
  namespace: agensys-demo-1
spec:
  {}
EOF
```

**Test connection (should fail now):**
```bash
kubectl exec -n agensys-demo-1 deploy/test-client -- \
  curl -v -m 5 http://test-server
```

**Expected output:**
```
* Connection timed out after 5000 milliseconds
curl: (28) Connection timed out after 5000 milliseconds
command terminated with exit code 28
```

✅ Connection should timeout (policy is blocking traffic).

**Check ztunnel logs for denials:**
```bash
kubectl logs -n istio-system -l app=ztunnel | grep RBAC_ACCESS_DENIED
```

**Expected:** Should see log entries about denied connections.

**Remove test policy:**
```bash
kubectl delete authorizationpolicy test-deny-all -n agensys-demo-1
```

**Verify connectivity restored:**
```bash
kubectl exec -n agensys-demo-1 deploy/test-client -- \
  curl -s http://test-server
```

Should work again and return nginx page.

### Test 4: Verify Per-Node Ztunnel

```bash
# Check which node test-server is on
TEST_SERVER_NODE=$(kubectl get pod -n agensys-demo-1 -l app=test-server \
  -o jsonpath='{.items[0].spec.nodeName}')

echo "Test server is on node: $TEST_SERVER_NODE"

# Get ztunnel pod on that node
ZTUNNEL_POD=$(kubectl get pod -n istio-system -l app=ztunnel \
  --field-selector spec.nodeName=$TEST_SERVER_NODE \
  -o jsonpath='{.items[0].metadata.name}')

echo "Ztunnel pod on that node: $ZTUNNEL_POD"

# Check ztunnel logs for test-server traffic
kubectl logs -n istio-system $ZTUNNEL_POD --tail=50 | grep test-server
```

Should see ztunnel processing traffic for test-server.

---

## Health Check Script

Create a comprehensive health check script to validate your installation:

```bash
#!/bin/bash
# ambient-health-check.sh

echo "=============================================================="
echo "  Istio Ambient Mode - Installation Health Check"
echo "=============================================================="
echo ""

EXIT_CODE=0

# Check 1: Istiod
echo "1. Checking Istiod Control Plane..."
ISTIOD_READY=$(kubectl get pods -n istio-system -l app=istiod \
  -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' | grep -c "True")
ISTIOD_TOTAL=$(kubectl get pods -n istio-system -l app=istiod --no-headers | wc -l)

if [ "$ISTIOD_READY" -eq "$ISTIOD_TOTAL" ] && [ "$ISTIOD_TOTAL" -gt 0 ]; then
  echo "   ✅ Istiod: $ISTIOD_READY/$ISTIOD_TOTAL pods ready"
else
  echo "   ❌ Istiod: $ISTIOD_READY/$ISTIOD_TOTAL pods ready (PROBLEM!)"
  EXIT_CODE=1
fi
echo ""

# Check 2: Ztunnel
echo "2. Checking Ztunnel Data Plane..."
ZTUNNEL_READY=$(kubectl get pods -n istio-system -l app=ztunnel \
  -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' | grep -c "True")
ZTUNNEL_TOTAL=$(kubectl get pods -n istio-system -l app=ztunnel --no-headers | wc -l)
NODE_COUNT=$(kubectl get nodes --no-headers | wc -l)

if [ "$ZTUNNEL_READY" -eq "$NODE_COUNT" ] && [ "$ZTUNNEL_TOTAL" -eq "$NODE_COUNT" ]; then
  echo "   ✅ Ztunnel: $ZTUNNEL_READY/$ZTUNNEL_TOTAL pods ready (1 per node)"
else
  echo "   ⚠️  Ztunnel: $ZTUNNEL_READY/$ZTUNNEL_TOTAL pods ready, expected $NODE_COUNT (1 per node)"
  EXIT_CODE=1
fi
echo ""

# Check 3: CNI
echo "3. Checking Istio CNI..."
CNI_READY=$(kubectl get pods -n istio-system -l app=istio-cni-node \
  -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' | grep -c "True")
CNI_TOTAL=$(kubectl get pods -n istio-system -l app=istio-cni-node --no-headers | wc -l)

if [ "$CNI_READY" -eq "$NODE_COUNT" ] && [ "$CNI_TOTAL" -eq "$NODE_COUNT" ]; then
  echo "   ✅ CNI: $CNI_READY/$CNI_TOTAL pods ready (1 per node)"
else
  echo "   ⚠️  CNI: $CNI_READY/$CNI_TOTAL pods ready, expected $NODE_COUNT (1 per node)"
  EXIT_CODE=1
fi
echo ""

# Check 4: Version consistency
echo "4. Checking Version Consistency..."
if command -v istioctl &> /dev/null; then
  istioctl version --short
  echo ""
else
  echo "   ⚠️  istioctl not found in PATH"
fi

# Check 5: Ambient-enabled namespaces
echo "5. Checking Ambient-Enabled Namespaces..."
AMBIENT_NS=$(kubectl get namespaces -l istio.io/dataplane-mode=ambient --no-headers | wc -l)
if [ "$AMBIENT_NS" -gt 0 ]; then
  echo "   ✅ Found $AMBIENT_NS namespace(s) with ambient mode:"
  kubectl get namespaces -l istio.io/dataplane-mode=ambient
else
  echo "   ℹ️  No namespaces with ambient mode enabled yet"
  echo "   Run: kubectl label namespace <namespace> istio.io/dataplane-mode=ambient"
fi
echo ""

# Check 6: CRDs
echo "6. Checking Istio CRDs..."
CRD_COUNT=$(kubectl get crd | grep istio.io | wc -l)
if [ "$CRD_COUNT" -gt 10 ]; then
  echo "   ✅ Found $CRD_COUNT Istio CRDs"
else
  echo "   ❌ Only found $CRD_COUNT Istio CRDs (expected 14+)"
  EXIT_CODE=1
fi
echo ""

# Check 7: Pod failures
echo "7. Checking for Failed Pods..."
FAILED_PODS=$(kubectl get pods -n istio-system | grep -v "Running\|Completed" | grep -v "NAME" || true)
if [ -z "$FAILED_PODS" ]; then
  echo "   ✅ No failed pods in istio-system"
else
  echo "   ❌ Found failed pods:"
  echo "$FAILED_PODS"
  EXIT_CODE=1
fi
echo ""

# Summary
echo "=============================================================="
echo "  Health Check Summary"
echo "=============================================================="
if [ $EXIT_CODE -eq 0 ]; then
  echo "✅ All checks passed! Istio Ambient Mode is properly installed."
  echo ""
  echo "Next steps:"
  echo "  1. Enable ambient mode for your namespace:"
  echo "     kubectl label namespace <your-namespace> istio.io/dataplane-mode=ambient"
  echo "  2. Deploy your applications"
  echo "  3. Apply Zero-Trust policies from policies/ directory"
else
  echo "❌ Some checks failed. Please review the output above."
  echo ""
  echo "Troubleshooting:"
  echo "  - Check pod logs: kubectl logs -n istio-system <pod-name>"
  echo "  - Describe failed pods: kubectl describe pod -n istio-system <pod-name>"
  echo "  - Review installation: istioctl analyze -n istio-system"
fi
echo ""

exit $EXIT_CODE
```

**Save and run:**
```bash
chmod +x ambient-health-check.sh
./ambient-health-check.sh
```

---

## Troubleshooting

### Issue 1: Ztunnel Pods Not Running

**Symptoms:**
- Ztunnel pods in CrashLoopBackOff or Error state
- DaemonSet shows desired > ready

**Check pod status:**
```bash
kubectl get pods -n istio-system -l app=ztunnel
kubectl describe pod -n istio-system -l app=ztunnel
```

**Check logs:**
```bash
kubectl logs -n istio-system -l app=ztunnel --tail=100
```

**Common causes:**
- Insufficient node resources (CPU/memory)
- CNI not installed properly
- Network plugin conflicts (e.g., Calico, Cilium)

**Solution:**
```bash
# Reinstall ztunnel
helm uninstall ztunnel -n istio-system
helm install ztunnel istio/ztunnel -n istio-system --wait

# Or with istioctl
istioctl uninstall --purge -y
istioctl install --set profile=ambient -y
```

### Issue 2: CNI Installation Failed

**Symptoms:**
- CNI pods not running
- Pods in namespace not getting mesh features

**Check CNI logs:**
```bash
kubectl logs -n istio-system -l app=istio-cni-node --tail=100
```

**Check CNI configuration:**
```bash
kubectl get configmap -n istio-system istio-cni-config -o yaml
```

**Solution:**
```bash
# Reinstall CNI
helm uninstall istio-cni -n istio-system
helm install istio-cni istio/cni -n istio-system --set profile=ambient --wait
```

### Issue 3: Istiod Not Starting

**Check istiod logs:**
```bash
kubectl logs -n istio-system -l app=istiod --tail=100
```

**Check istiod events:**
```bash
kubectl describe pod -n istio-system -l app=istiod
```

**Common causes:**
- CRDs not installed
- Insufficient resources
- Configuration errors

**Solution:**
```bash
# Check CRDs exist
kubectl get crd | grep istio.io

# Reinstall base if needed
helm uninstall istio-base -n istio-system
helm install istio-base istio/base -n istio-system --wait

# Reinstall istiod
helm uninstall istiod -n istio-system
helm install istiod istio/istiod -n istio-system --wait
```

### Issue 4: Namespace Not in Ambient Mode

**Symptoms:**
- Pods get sidecar injected (istio-proxy container)
- OR pods don't get mesh features

**Verify namespace label:**
```bash
kubectl get namespace agensys-demo-1 -o jsonpath='{.metadata.labels}'
```

**Solution:**
```bash
# Remove sidecar injection label if present
kubectl label namespace agensys-demo-1 istio-injection-

# Add ambient mode label
kubectl label namespace agensys-demo-1 istio.io/dataplane-mode=ambient --overwrite

# Restart pods to apply changes
kubectl rollout restart deployment -n agensys-demo-1
```

### Issue 5: mTLS Not Working

**Check istiod is running:**
```bash
kubectl get pods -n istio-system -l app=istiod
```

**Check ztunnel can connect to istiod:**
```bash
istioctl proxy-status
```

**Check ztunnel configuration:**
```bash
kubectl exec -n istio-system -l app=ztunnel -- \
  curl -s localhost:15000/config_dump | grep -A 10 "workload"
```

**Solution:**
```bash
# Restart ztunnel pods
kubectl rollout restart daemonset ztunnel -n istio-system

# Check connectivity
kubectl exec -n istio-system deploy/istiod -- curl -s localhost:15014/ready
```

### Issue 6: Version Mismatch

**Check versions:**
```bash
istioctl version
```

**If versions don't match:**
```bash
# Upgrade all components to same version
helm upgrade istio-base istio/base -n istio-system
helm upgrade istiod istio/istiod -n istio-system
helm upgrade istio-cni istio/cni -n istio-system
helm upgrade ztunnel istio/ztunnel -n istio-system

# Or with istioctl
istioctl upgrade --set profile=ambient
```

### Issue 7: Cleanup and Reinstall

**If all else fails, perform a clean reinstall:**

```bash
# Delete test applications
kubectl delete namespace agensys-demo-1

# Uninstall Istio (Helm method)
helm uninstall ztunnel -n istio-system
helm uninstall istio-cni -n istio-system
helm uninstall istiod -n istio-system
helm uninstall istio-base -n istio-system

# OR uninstall Istio (istioctl method)
istioctl uninstall --purge -y

# Delete namespace
kubectl delete namespace istio-system

# Wait 30 seconds for cleanup
sleep 30

# Reinstall from Step 1
```

---

## Verification Checklist

After installation, verify the following:

- ✅ **Istiod running**: 1 pod, status Running
- ✅ **Ztunnel running**: 1 pod per node, all Running
- ✅ **CNI running**: 1 pod per node, all Running
- ✅ **Versions match**: `istioctl version` shows consistent versions
- ✅ **CRDs installed**: 14+ Istio CRDs present
- ✅ **No failed pods**: All pods in istio-system are Running
- ✅ **Namespace labeled**: Test namespace has `istio.io/dataplane-mode=ambient`
- ✅ **No sidecars**: Test pods have ONLY application container
- ✅ **Connectivity works**: Pod-to-pod communication succeeds
- ✅ **Policies enforced**: Authorization policies can block traffic
- ✅ **mTLS enabled**: Ztunnel logs show encrypted connections

---

## Next Steps

Once Istio Ambient Mode is installed and verified:

1. **Deploy your applications** to the ambient-enabled namespace
2. **Apply Zero-Trust policies** from the `policies/` directory
3. **Test policy enforcement** using `connectivity-test.sh`
4. **Monitor with observability tools** (Prometheus, Grafana, Kiali)

See [README.md](README.md) for policy deployment and [TECHNICAL_DESCRIPTION.md](TECHNICAL_DESCRIPTION.md) for detailed policy explanations.

---

## Additional Resources

- [Istio Ambient Mode Documentation](https://istio.io/latest/docs/ambient/)
- [Istio Installation Guide](https://istio.io/latest/docs/setup/install/)
- [Istio Helm Charts](https://github.com/istio/istio/tree/master/manifests/charts)
- [Troubleshooting Guide](https://istio.io/latest/docs/ops/diagnostic-tools/)
- [Authorization Policies](https://istio.io/latest/docs/reference/config/security/authorization-policy/)

---

## Support

For issues with:
- **Istio installation**: Check [Istio Discuss](https://discuss.istio.io/)
- **Zero-Trust policies**: See [TECHNICAL_DESCRIPTION.md](TECHNICAL_DESCRIPTION.md)
- **Policy testing**: Run `connectivity-test.sh` and check logs

For bugs or feature requests related to these policies, please open an issue in this repository.
