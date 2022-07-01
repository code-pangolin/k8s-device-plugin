#!/bin/bash
version=0.12.2
docker build -t storswift/k8s-device-plugin:${version} -f deployments/container/Dockerfile.ubuntu .
docker push storswift/k8s-device-plugin:${version}