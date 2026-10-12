#!/usr/bin/env bash
set -eu
cd "$(dirname "$0")/.."
LOGE() { :; }
eval "$(sed -n '/^validate_certificate_domain() {/,/^}$/p' s-ui.sh)"
for domain in example.com ttsharett.dpdns.org xn--example.test; do
    validate_certificate_domain "$domain" || exit 1
done
for domain in '../escape' 'a/b' 'a..b' '-bad.test' 'x;cmd' ''; do
    if validate_certificate_domain "$domain"; then echo "Unsafe domain accepted"; exit 1; fi
done
SUI_HOME=/usr/local/s-ui
path_has_symlink_component() { [[ "$1" == *redirect* ]]; }
getent() { return 1; }
install() { :; }
eval "$(sed -n '/^prepare_certificate_directory() {/,/^}$/p' s-ui.sh)"
prepare_certificate_directory "$SUI_HOME/cert/example.com"
if prepare_certificate_directory /root/cert/example.com; then exit 1; fi
if prepare_certificate_directory "$SUI_HOME/cert/redirect"; then exit 1; fi
echo 'Certificate domain and directory guards: OK'