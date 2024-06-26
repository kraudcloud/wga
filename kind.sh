#!/usr/bin/env bash

docker build . -t ghcr.io/kraudcloud/wga:latest
kind load docker-image ghcr.io/kraudcloud/wga:latest

kubectl create secret generic wga-secret --from-literal=privateKey=$(head -c 32 /dev/urandom | base64)

helm uninstall wga
helm install wga ./charts/wga \
    --set endpoint.image.pullPolicy=Never \
    --set endpoint.image.tag=latest \
    --set endpoint.privateKeySecretName=wga-secret \
    --set endpoint.address=192.168.1.10 \
    --set endpoint.clientCIDR=192.168.1.0/24 \
    --set endpoint.allowedIPs=192.168.1.0/24 \
    --set clusterClient.enable=true \
    --set dns.enabled=true \
    --set dns.address=192.168.1.53 \
    "$@"
