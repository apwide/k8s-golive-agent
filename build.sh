#!/bin/bash
docker build . --no-cache --pull --progress=plain --tag docker.apwide.com/k8s-golive-agent:latest
