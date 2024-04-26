#/usr/bin/env bash

docker build . -t ghcr.io/kraudcloud/wga:latest
kind load docker-image ghcr.io/kraudcloud/wga:latest
helm uninstall wga
helm install wga ./charts/wga --set version=latest

kubectl wait --for=condition=Ready pod -l app=wireguard --timeout=30s
kubectl logs -l app=wireguard --tail=100 -f
