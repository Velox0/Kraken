#!/usr/bin/env bash
set -euo pipefail

echo "Restarting nginx"
systemctl restart nginx
