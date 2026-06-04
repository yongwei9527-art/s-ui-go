#!/usr/bin/env sh
set -eu

export SUI_DB_FOLDER="${SUI_DB_FOLDER:-db}"
export SUI_DEBUG="${SUI_DEBUG:-false}"

mkdir -p "$SUI_DB_FOLDER" cert logs

if [ -x ./sui ]; then
  exec ./sui "$@"
fi

exec /app/sui "$@"