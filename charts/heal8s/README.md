# heal8s Helm Chart

Helm chart for deploying the heal8s Kubernetes Operator.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- kubectl configured to access your cluster

## Installation

### Basic Installation

```bash
helm install heal8s charts/heal8s/ \
  --create-namespace \
  --namespace heal8s-system
```

### Production Installation

For production deployments, it's recommended to use custom values:

```bash
helm install heal8s charts/heal8s/ \
  --create-namespace \
  --namespace heal8s-system \
  --values values-production.yaml
```

## Configuration

### Key Configuration Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.image.repository` | Operator image repository | `heal8s/operator` |
| `operator.image.tag` | Operator image tag | `v0.1.0` |
| `operator.replicaCount` | Number of operator replicas | `1` |
| `operator.resources.limits.memory` | Memory limit | `256Mi` |
| `operator.resources.limits.cpu` | CPU limit | `200m` |
| `operator.metrics.enabled` | Enable Prometheus metrics | `true` |
| `alertRouting` | Alert routing configuration | See values.yaml |

### Alert Routing Configuration

Configure how alerts map to remediation actions:

```yaml
alertRouting:
  KubePodOOMKilled:
    action: IncreaseMemory
    params:
      memoryIncreasePercent: "25"
      maxMemory: "2Gi"
  
  KubeHpaMaxedOut:
    action: ScaleUp
    params:
      scaleUpPercent: "50"
      maxReplicas: "10"
```

### Resource Limits

For production workloads, adjust resource limits:

```yaml
operator:
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 200m
      memory: 256Mi
```

### High Availability

Enable leader election for HA (default: `true`). The chart grants RBAC for `coordination.k8s.io/leases` so the operator can acquire the leader lock.

```yaml
operator:
  replicaCount: 3
  leaderElection:
    enabled: true
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: heal8s
          topologyKey: kubernetes.io/hostname
```

## Upgrading

```bash
helm upgrade heal8s charts/heal8s/ \
  --namespace heal8s-system \
  --values custom-values.yaml
```

## Uninstalling

```bash
helm uninstall heal8s --namespace heal8s-system
```

**Note:** This will not remove the CRDs. To remove CRDs:

```bash
kubectl delete crd remediations.k8shealer.k8s-healer.io
```

## Monitoring

### Prometheus Metrics

The operator exposes Prometheus metrics on port 8080 (configurable):

```yaml
operator:
  metrics:
    enabled: true
    port: 8080
```

Key metrics:
- `heal8s_alerts_received_total` - Total alerts received
- `heal8s_remediations_created_total` - Total remediations created
- `heal8s_remediations_succeeded_total` - Successful remediations
- `heal8s_remediations_failed_total` - Failed remediations
- `heal8s_remediation_duration_seconds` - Remediation duration

### ServiceMonitor

If you're using Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: heal8s-operator
  namespace: heal8s-system
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: heal8s
      app.kubernetes.io/component: operator
  endpoints:
  - port: metrics
    interval: 30s
```

## Examples

### Development Installation

```bash
helm install heal8s charts/heal8s/ \
  --create-namespace \
  --namespace heal8s-system \
  --set operator.image.pullPolicy=IfNotPresent \
  --set operator.leaderElection.enabled=false
```

### Production with Custom Alert Routing

```bash
cat <<EOF > production-values.yaml
operator:
  replicaCount: 2
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 200m
      memory: 256Mi
  
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  
  tolerations:
  - key: node-role.kubernetes.io/control-plane
    operator: Exists
    effect: NoSchedule

alertRouting:
  KubePodOOMKilled:
    action: IncreaseMemory
    params:
      memoryIncreasePercent: "50"
      maxMemory: "4Gi"
  
  KubeHpaMaxedOut:
    action: ScaleUp
    params:
      scaleUpPercent: "100"
      maxReplicas: "20"
  
  KubePodCrashLooping:
    action: RollbackImage
    params:
      rollbackMaxRevisions: "5"
EOF

helm install heal8s charts/heal8s/ \
  --create-namespace \
  --namespace heal8s-system \
  --values production-values.yaml
```

## Troubleshooting

### Check operator logs

```bash
kubectl logs -n heal8s-system -l app.kubernetes.io/name=heal8s -f
```

### Verify CRDs

```bash
kubectl get crd remediations.k8shealer.k8s-healer.io
```

### Check webhook service

```bash
kubectl get svc -n heal8s-system
kubectl get endpoints -n heal8s-system
```

### Test webhook connectivity

```bash
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -v http://heal8s-operator.heal8s-system.svc.cluster.local:8082/webhooks/alertmanager
```

## Support

For issues and questions, please visit:
- GitHub Issues: https://github.com/heal8s/heal8s/issues
- Documentation: https://github.com/heal8s/heal8s/tree/main/docs
