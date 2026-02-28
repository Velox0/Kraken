#!/usr/bin/env bash
set -euo pipefail

echo "Restarting pm2"
pm2 restart all
