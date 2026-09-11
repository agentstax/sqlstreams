#!/usr/bin/env bash
# Writes the run's fingerprint as one JSON document on stdout: the facts the
# checker cannot see from inside its container -- the library commit, the
# host, the Docker engine, and the image the postgres service is running.
# The checker adds the Go and Postgres facts it can read itself. Run from
# .bench with the compose stack up.
set -euo pipefail

library_sha=$(git rev-parse HEAD)
library_dirty=false
if [ -n "$(git status --porcelain)" ]; then library_dirty=true; fi

host_os=$(uname -sr)
case "$(uname -s)" in
Darwin)
  host_cpu=$(sysctl -n machdep.cpu.brand_string)
  host_cores=$(sysctl -n hw.ncpu)
  host_memory_bytes=$(sysctl -n hw.memsize)
  ;;
*)
  host_cpu=$(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ *//')
  host_cores=$(nproc)
  host_memory_bytes=$(( $(awk '/MemTotal/ {print $2}' /proc/meminfo) * 1024 ))
  ;;
esac

execution=${1:-compose}
postgres_image=external
docker_cpus=0
docker_memory_bytes=0
docker_version=""
runtime='{}'
if [ "$execution" = compose ]; then
  postgres_image=$(docker inspect --format '{{.Config.Image}}' "$(docker compose ps -q postgres)")
  docker_cpus=$(docker info --format '{{.NCPU}}')
  docker_memory_bytes=$(docker info --format '{{.MemTotal}}')
  docker_version=$(docker info --format '{{.ServerVersion}}')
else
  runtime="{\"GOMAXPROCS\": \"${GOMAXPROCS:-}\", \"GOGC\": \"${GOGC:-}\", \"GOMEMLIMIT\": \"${GOMEMLIMIT:-}\"}"
fi

cat <<JSON
{
  "execution": "$execution",
  "runtime": $runtime,
  "library_sha": "$library_sha",
  "library_dirty": $library_dirty,
  "postgres_image": "$postgres_image",
  "host": {
    "os": "$host_os",
    "cpu": "$host_cpu",
    "cores": $host_cores,
    "memory_bytes": $host_memory_bytes
  },
  "docker": {
    "cpus": $docker_cpus,
    "memory_bytes": $docker_memory_bytes,
    "version": "$docker_version"
  }
}
JSON
