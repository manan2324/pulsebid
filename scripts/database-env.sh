#!/usr/bin/env bash
# Source this helper; read only the one dotenv value needed by native DB tools.
# Treat the value as data, never evaluate the environment file as shell code.
if [[ -z ${DATABASE_URL:-} ]]; then
  env_file=${COMPOSE_ENV_FILE:-.env}
  [[ -f $env_file ]] || env_file=.env.example
  while IFS= read -r line || [[ -n $line ]]; do
    line=${line%$'\r'}
    if [[ $line == DATABASE_URL=* ]]; then
      DATABASE_URL=${line#DATABASE_URL=}
      DATABASE_URL=${DATABASE_URL#\"}; DATABASE_URL=${DATABASE_URL%\"}
      DATABASE_URL=${DATABASE_URL#\'}; DATABASE_URL=${DATABASE_URL%\'}
      export DATABASE_URL
      break
    fi
  done < "$env_file"
fi
