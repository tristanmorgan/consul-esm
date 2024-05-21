#!/bin/sh

# export DOCKER_HOST=ssh://tristan@inspirone.node.home.consul
unset DOCKER_HOST

TS_VAR=$(date +%s)
docker build --provenance false --platform=linux/amd64 -t registry.service.home.consul/consul-esm:$TS_VAR-amd64 .
docker push registry.service.home.consul/consul-esm:$TS_VAR-amd64
docker build --provenance false --platform=linux/arm64/v8 -t registry.service.home.consul/consul-esm:$TS_VAR-arm64 .
docker push registry.service.home.consul/consul-esm:$TS_VAR-arm64

docker manifest create registry.service.home.consul/consul-esm:$TS_VAR --amend registry.service.home.consul/consul-esm:$TS_VAR-arm64 --amend registry.service.home.consul/consul-esm:$TS_VAR-amd64
docker manifest push registry.service.home.consul/consul-esm:$TS_VAR

docker manifest rm registry.service.home.consul/consul-esm:latest
docker manifest create registry.service.home.consul/consul-esm:latest --amend registry.service.home.consul/consul-esm:$TS_VAR-arm64 --amend registry.service.home.consul/consul-esm:$TS_VAR-amd64
docker manifest push registry.service.home.consul/consul-esm:latest
