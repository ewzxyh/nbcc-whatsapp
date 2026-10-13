#!/bin/sh
set -eu
if [ "${RENEWED_LINEAGE:-}" = "/etc/letsencrypt/live/nbcc.example.com" ]; then
    /usr/sbin/nginx -t
    /bin/systemctl reload nginx
fi
