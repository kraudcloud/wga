#!/bin/sh

eval $(minikube docker-env)
docker build . -t ghcr.io/kraudcloud/wga:latest
helm uninstall wga
helm install wga ./charts/wga \
    --set endpoint.image.pullPolicy=Never \
    --set endpoint.image.tag=latest \
    --set endpoint.address=192.168.1.10 \
    --set endpoint.clientCIDR=192.168.1.0/24 \
    --set endpoint.allowedIPs=192.168.1.0/24 \
    --set clusterClient.enable=true \
    --set dns.enabled=true \
    --set dns.address=192.168.1.53 \
    "$@"
