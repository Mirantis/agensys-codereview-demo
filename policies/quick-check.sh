#!/bin/bash
# ==============================================================================
# Quick Ambient Mode Setup Validation Script
# File: quick-check.sh
# ==============================================================================
#
# PURPOSE:
#   Quickly validate that Istio Ambient Mode and Zero-Trust policies are
#   properly configured and operational.
#
# USAGE:
#   chmod +x quick-check.sh
#   ./quick-check.sh
#
# WHAT IT CHECKS:
#   - Authorization policies are applied
#   - Service entries exist
#   - Namespace is in ambient mode
#   - Ztunnel pods are running
#   - Workloads are deployed
#   - Recent policy enforcement events
#
# ==============================================================================

NAMESPACE="agensys-demo-1"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=============================================================="
echo "  Istio Ambient Mode - Quick Configuration Check"
echo "  Namespace: ${NAMESPACE}"
echo -e "==============================================================${NC}"
echo ""

# ==============================================================================
# Check 1: Authorization Policies
# ==============================================================================

echo -e "${YELLOW}=== Authorization Policies ===${NC}"
POLICY_COUNT=$(kubectl get authorizationpolicies -n $NAMESPACE --no-headers 2>/dev/null | wc -l)

if [ $POLICY_COUNT -eq 0 ]; then
    echo -e "${RED}❌ No authorization policies found!${NC}"
else
    echo -e "${GREEN}✅ Found $POLICY_COUNT authorization policies${NC}"
    kubectl get authorizationpolicies -n $NAMESPACE
fi

echo ""

# ==============================================================================
# Check 2: Service Entries
# ==============================================================================

echo -e "${YELLOW}=== Service Entries (External APIs) ===${NC}"
SE_COUNT=$(kubectl get serviceentries -n $NAMESPACE --no-headers 2>/dev/null | wc -l)

if [ $SE_COUNT -eq 0 ]; then
    echo -e "${RED}⚠️  No service entries found${NC}"
else
    echo -e "${GREEN}✅ Found $SE_COUNT service entries${NC}"
    kubectl get serviceentries -n $NAMESPACE
fi

echo ""

# ==============================================================================
# Check 3: Namespace Ambient Mode
# ==============================================================================

echo -e "${YELLOW}=== Namespace Ambient Mode Status ===${NC}"
DATAPLANE_MODE=$(kubectl get namespace $NAMESPACE -o jsonpath='{.metadata.labels.istio\.io/dataplane-mode}' 2>/dev/null)

if [ "$DATAPLANE_MODE" == "ambient" ]; then
    echo -e "${GREEN}✅ Namespace is in ambient mode${NC}"
else
    echo -e "${RED}❌ Namespace is NOT in ambient mode (current: ${DATAPLANE_MODE:-none})${NC}"
    echo "   To enable: kubectl label namespace $NAMESPACE istio.io/dataplane-mode=ambient"
fi

echo ""

# ==============================================================================
# Check 4: Ztunnel Status
# ==============================================================================

echo -e "${YELLOW}=== Ztunnel Pods (Ambient Data Plane) ===${NC}"
ZTUNNEL_COUNT=$(kubectl get pods -n istio-system -l app=ztunnel --no-headers 2>/dev/null | grep Running | wc -l)

if [ $ZTUNNEL_COUNT -eq 0 ]; then
    echo -e "${RED}❌ No ztunnel pods running!${NC}"
    echo "   Ambient mode requires ztunnel DaemonSet"
else
    echo -e "${GREEN}✅ $ZTUNNEL_COUNT ztunnel pod(s) running${NC}"
    kubectl get pods -n istio-system -l app=ztunnel
fi

echo ""

# ==============================================================================
# Check 5: Workloads in Namespace
# ==============================================================================

echo -e "${YELLOW}=== Workloads in $NAMESPACE ===${NC}"
POD_COUNT=$(kubectl get pods -n $NAMESPACE --no-headers 2>/dev/null | wc -l)

if [ $POD_COUNT -eq 0 ]; then
    echo -e "${RED}⚠️  No pods found in namespace${NC}"
else
    echo -e "${GREEN}✅ Found $POD_COUNT pod(s)${NC}"
    kubectl get pods -n $NAMESPACE
fi

echo ""

# ==============================================================================
# Check 6: Istiod Status
# ==============================================================================

echo -e "${YELLOW}=== Istiod (Control Plane) Status ===${NC}"
ISTIOD_COUNT=$(kubectl get pods -n istio-system -l app=istiod --no-headers 2>/dev/null | grep Running | wc -l)

if [ $ISTIOD_COUNT -eq 0 ]; then
    echo -e "${RED}❌ No istiod pods running!${NC}"
else
    echo -e "${GREEN}✅ $ISTIOD_COUNT istiod pod(s) running${NC}"
    kubectl get pods -n istio-system -l app=istiod
fi

echo ""

# ==============================================================================
# Check 7: Recent Policy Enforcement Events
# ==============================================================================

echo -e "${YELLOW}=== Recent Policy Enforcement Events ===${NC}"
echo "Checking ztunnel logs for recent RBAC decisions..."
echo ""

DENIALS=$(kubectl logs -n istio-system -l app=ztunnel --tail=100 2>/dev/null | grep RBAC_ACCESS_DENIED | tail -10)

if [ -z "$DENIALS" ]; then
    echo -e "${GREEN}ℹ️  No recent policy denials (all traffic is allowed or no traffic attempted)${NC}"
else
    echo -e "${YELLOW}Recent denied connections:${NC}"
    echo "$DENIALS"
fi

echo ""

# ==============================================================================
# Summary
# ==============================================================================

echo -e "${BLUE}=============================================================="
echo "  Summary"
echo -e "==============================================================${NC}"
echo ""

ISSUES=0

[ $POLICY_COUNT -eq 0 ] && ((ISSUES++)) && echo -e "${RED}❌ Missing authorization policies${NC}"
[ "$DATAPLANE_MODE" != "ambient" ] && ((ISSUES++)) && echo -e "${RED}❌ Namespace not in ambient mode${NC}"
[ $ZTUNNEL_COUNT -eq 0 ] && ((ISSUES++)) && echo -e "${RED}❌ Ztunnel not running${NC}"
[ $ISTIOD_COUNT -eq 0 ] && ((ISSUES++)) && echo -e "${RED}❌ Istiod not running${NC}"
[ $POD_COUNT -eq 0 ] && ((ISSUES++)) && echo -e "${YELLOW}⚠️  No workloads deployed${NC}"

if [ $ISSUES -eq 0 ]; then
    echo -e "${GREEN}✅ All checks passed! Ambient mode is properly configured.${NC}"
    echo ""
    echo "Next steps:"
    echo "  - Run connectivity-test.sh to validate policy enforcement"
    echo "  - Deploy your agents and MCP servers"
    echo "  - Monitor ztunnel logs: kubectl logs -n istio-system -l app=ztunnel -f"
else
    echo -e "${RED}⚠️  Found $ISSUES issue(s) that need attention${NC}"
    echo ""
    echo "Troubleshooting:"
    echo "  - Check README.md for detailed setup instructions"
    echo "  - Verify Istio installation: istioctl version"
    echo "  - Review namespace labels: kubectl get ns $NAMESPACE -o yaml"
fi

echo ""
