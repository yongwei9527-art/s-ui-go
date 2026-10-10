#!/usr/bin/env bash
set -eu
cd "$(dirname "$0")/.."
block="$(sed -n '/^_health_pid=/,/^done$/p' install-linux.sh)"
[[ -n "$block" ]] || { echo 'Missing health validation'; exit 1; }
for scenario in stable failed restarted missing; do
    if (
        SERVICE_NAME=s-ui
        tick=0
        sleep() { tick=$((tick + 1)); }
        journalctl() { :; }
        die() { exit 42; }
        systemctl() {
            if [[ "$1" == show ]]; then
                if [[ "$scenario" == missing ]]; then echo 0
                elif [[ "$scenario" == restarted && "$tick" -ge 3 ]]; then echo 101
                else echo 100; fi
            elif [[ "$1" == is-active ]]; then
                [[ "$scenario" != failed || "$tick" -lt 3 ]]
            else return 1; fi
        }
        eval "$block"
        [[ "$tick" == 15 ]]
    ); then result=0; else result=$?; fi
    if [[ "$scenario" == stable ]]; then [[ "$result" == 0 ]]
    else [[ "$result" == 42 ]]; fi
    echo "$scenario: OK"
done
