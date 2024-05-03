#!/bin/sh

eval $(minikube docker-env)
docker build . -t ghcr.io/kraudcloud/wga:latest
helm uninstall wga
helm install wga ./charts/wga --set logLevel=0 --set image.tag=latest
