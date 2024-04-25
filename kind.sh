#/usr/bin/env bash

docker build . -t ghcr.io/kraudcloud/wga:latest
kind load docker-image ghcr.io/kraudcloud/wga:latest
helm uninstall wga
helm install wga ./charts/wga --set version=latest
