#!/usr/bin/env bash
set -eu
cd "$(dirname "$0")/.."
# Mock certificate operations; cancellation after a domain is requested must not
# remove certificates belonging to any domain.
block="$(sed -n '/^ssl_cert_issue_CF() {/,/^}$/p' s-ui.sh)"
block="${block//\~\/.acme.sh\/acme.sh/acme_mock}"
LOGD() { :; }; LOGI() { :; }; LOGE() { :; }
confirm() { return 0; }
install_acme() { :; }
acme_mock() { :; }
validate_certificate_domain() { return 1; }
rm() { echo 'Certificate deletion attempted'; exit 99; }
mkdir() { echo 'Directory creation before validation'; exit 99; }
SUI_HOME=/usr/local/s-ui
# Avoid trying to install the real acme client during this mock.
command() { return 0; }
eval "$block"
if ssl_cert_issue_CF <<< $'1\nexample.test\n'; then result=0; else result=$?; fi
[[ "$result" == 1 ]] || { echo "Unexpected result: $result"; exit 1; }
echo 'Certificate cancellation never deletes existing files: OK'
