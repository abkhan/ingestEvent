# Kubernetes Deployment for Fulcrum

This directory contains Kubernetes manifests for deploying the Fulcrum ingestion service and PostgreSQL on Minikube.

## Image

The deployment uses the GitHub Container Registry image:

- `ghcr.io/abkhan/ingestevent:release_1`

Build and push the image before applying the manifests.

## Apply the manifests

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/postgres.yaml
kubectl apply -f k8s/fulcrum-deployment.yaml
kubectl apply -f k8s/fulcrum-service.yaml
```

## Service access

The service is exposed as a LoadBalancer on port `8083`.

- Minikube URL: `http://$(minikube ip):8083`
- If using `minikube tunnel`, access the assigned LoadBalancer IP on port `8083`.
- Or use port-forwarding if you prefer local access:

```bash
kubectl port-forward svc/fulcrum-service 8083:8083
```

## Verification

Use the test script at the repository root:

```bash
chmod +x test-service.sh
./test-service.sh http://$(minikube ip):8083
```

If you use a different API key than `default-key`, pass it as the second argument:

```bash
./test-service.sh http://$(minikube ip):30080 my-api-key
```
