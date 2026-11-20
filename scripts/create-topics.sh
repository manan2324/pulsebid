#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
# COMPOSE is a command plus arguments, e.g. 'docker compose'. No eval.
read -r -a compose <<< "${COMPOSE:-docker compose}"
env_file=${COMPOSE_ENV_FILE:-.env}
if [[ ! -f $env_file ]]; then env_file=.env.example; fi
compose+=(--env-file "$env_file" -f deploy/docker-compose.yml)

rpk() {
  "${compose[@]}" exec -T redpanda rpk "$@" -X brokers=redpanda:29092
}

# List first: a broker failure must fail the command, not look like a missing topic.
existing=$(rpk topic list)
for spec in 'pulsebid.events.v1 6' 'pulsebid.events.dlq.v1 1'; do
  read -r topic partitions <<< "$spec"
  if awk -v topic="$topic" '$1 == topic { found=1 } END { exit !found }' <<< "$existing"; then
    echo "Topic already exists: $topic"
  else
    rpk topic create "$topic" --partitions "$partitions" --replicas 1 \
      --topic-config retention.ms=604800000
  fi
  rpk topic describe "$topic"
done
