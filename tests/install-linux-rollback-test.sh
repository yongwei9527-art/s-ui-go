#!/usr/bin/env bash
set -eu
cd "$(dirname "$0")/.."
block="$(sed -n '/^restore_upgrade() {/,/^}$/p' install-linux.sh)"
[[ -n "$block" ]]
for scenario in stop-failed still-active; do
  if (
    SERVICE_NAME=test
    INSTALL_DIR=/nonexistent
    BACKUP_DIR=/nonexistent
    say() { :; }
    rollback_error() { ROLLBACK_FAILED=1; }
    systemctl() { [[ "$scenario" != stop-failed ]]; }
    service_is_inactive() { return 1; }
    restore_file_from_backup() { exit 99; }
    rm() { exit 99; }
    cp() { exit 99; }
    eval "$block"
    restore_upgrade
  ); then result=0; else result=$?; fi
  [[ "$result" == 1 ]] || { echo "Unsafe rollback: $scenario ($result)"; exit 1; }
  echo "$scenario: destructive recovery refused"
done