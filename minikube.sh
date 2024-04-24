#!/bin/sh

eval $(minikube docker-env)
docker build . -t ghcr.io/kraudcloud/wga:latest
helm uninstall wga
helm install wga ./charts/wga --set version=latest
