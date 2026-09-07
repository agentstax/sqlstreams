#!/usr/bin/env bash
# Samples the CPU of every container in the lab's compose project from the
# host, one JSON line each, about once a second until killed: the
# generator's own load is part of every result, and a container compose
# capped is judged against its cap. Run from bench/reliability with the
# compose stack up.
set -euo pipefail

# a TERM ends the loop after the iteration in flight, so the last line is
# whole when the caller's wait returns
stop=0
trap 'stop=1' TERM INT
project=$(docker compose config --format json | sed -n 's/.*"name": *"\([^"]*\)".*/\1/p' | head -1)
while [ "$stop" = 0 ]; do
  at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  docker stats --no-stream --format '{{.Name}} {{.CPUPerc}}' | while read -r name cpu; do
    read -r container_project service nano_cpus <<<"$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}} {{index .Config.Labels "com.docker.compose.service"}} {{.HostConfig.NanoCpus}}' "$name" 2>/dev/null || echo 'none none 0')"
    if [ "$container_project" != "$project" ]; then continue; fi
    cpus=$(awk "BEGIN { print ${nano_cpus:-0} / 1000000000 }")
    printf '{"at":"%s","name":"%s","service":"%s","cpu_percent":%s,"cpus":%s}\n' "$at" "$name" "$service" "${cpu%\%}" "$cpus"
  done
  sleep 1
done
