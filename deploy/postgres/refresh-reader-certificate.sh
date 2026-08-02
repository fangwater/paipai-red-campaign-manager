#!/usr/bin/env bash
set -euo pipefail

domain="${1:-pangutech.online}"
source_dir="/etc/letsencrypt/live/${domain}"
target_dir="/etc/postgresql/16/main/ssl"

install -d -o postgres -g postgres -m 0750 "${target_dir}"
install -o postgres -g postgres -m 0644 "${source_dir}/fullchain.pem" "${target_dir}/paipai-fullchain.pem"
install -o postgres -g postgres -m 0600 "${source_dir}/privkey.pem" "${target_dir}/paipai-privkey.pem"
pg_ctlcluster 16 main reload
