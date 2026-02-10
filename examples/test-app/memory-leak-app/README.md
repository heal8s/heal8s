# Memory Leak Simulator

Realistic memory leak application for testing heal8s.

## Features

- Gradually leaks memory at configurable rate
- HTTP endpoints for health checks and metrics
- Configurable via environment variables
- Prometheus-compatible metrics endpoint

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `LEAK_RATE_MB` | 10 | MB to leak per interval |
| `LEAK_INTERVAL` | 5s | Time between leaks |
| `MAX_LEAK_MB` | 500 | Maximum memory to leak |
| `PORT` | 8080 | HTTP server port |

## Endpoints

- `GET /` - Status page with memory stats
- `GET /health` - Health check (returns 200 OK)
- `GET /metrics` - Prometheus metrics

## Building

```bash
docker build -t memory-leak-simulator:latest .
```

## Loading to Kind

```bash
kind load docker-image memory-leak-simulator:latest --name heal8s-dev
```

## Deploying

```bash
kubectl apply -f ../memory-leak-deployment.yaml
```

## Testing

Watch memory usage:

```bash
kubectl top pods -n test-app
```

Watch for OOM kills:

```bash
kubectl get events -n test-app --watch | grep OOM
```

Check metrics:

```bash
kubectl port-forward -n test-app svc/memory-leak-app 8080:80
curl http://localhost:8080/metrics
```
