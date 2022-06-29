#!/bin/bash
docker build -t storswift/k8s-device-plugin:devel -f deployments/container/Dockerfile.ubuntu .
docker push storswift/k8s-device-plugin:devel