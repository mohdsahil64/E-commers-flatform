#!/usr/bin/env bash
# =====================================================================
# CARLITOS - one-command deploy script (run on the VPS)
# ---------------------------------------------------------------------
# After you `git push` from your Mac, just run `deploy` on the VPS.
# It pulls the latest code, rebuilds only what changed, and restarts
# the backend + reloads nginx.
#
# It is SMART: it rebuilds the Go backend only if backend files changed,
# and rebuilds the frontends only if frontend/package files changed —
# so most deploys are fast.
# =====================================================================
set -euo pipefail

REPO="/www/wwwroot/carlitos"
NGINX="/www/server/nginx/sbin/nginx"

cd "$REPO"

echo "==> [1/5] Fetching latest code..."
git config --global --add safe.directory "$REPO" 2>/dev/null || true
BEFORE="$(git rev-parse HEAD)"
git pull origin main
AFTER="$(git rev-parse HEAD)"

if [ "$BEFORE" = "$AFTER" ]; then
  echo "==> Already up to date. Nothing to deploy."
  exit 0
fi

# What changed between the old and new commit?
CHANGED="$(git diff --name-only "$BEFORE" "$AFTER")"
echo "==> Changed files:"
echo "$CHANGED" | sed 's/^/      /'

# ---- Backend: rebuild + restart only if backend/ changed ----
if echo "$CHANGED" | grep -q '^backend/'; then
  echo "==> [2/5] Backend changed — rebuilding Go binary..."
  ( cd backend && go build -buildvcs=false -o carlitos-api ./cmd/api )
  echo "==> [3/5] Restarting backend service..."
  systemctl restart carlitos-api
else
  echo "==> [2/5] No backend changes — skipping Go rebuild."
  echo "==> [3/5] Backend restart not needed."
fi

# ---- Frontends: rebuild only if apps/ or packages/ or root deps changed ----
if echo "$CHANGED" | grep -qE '^(apps/|packages/|package\.json|package-lock\.json)'; then
  echo "==> [4/5] Frontend changed — installing deps + building all apps..."
  npm install
  npm run build:all
else
  echo "==> [4/5] No frontend changes — skipping build."
fi

# ---- Always reload nginx (cheap, picks up any new static files) ----
echo "==> [5/5] Reloading nginx..."
"$NGINX" -t && "$NGINX" -s reload

echo ""
echo "==> ✅ Deploy complete.  $BEFORE -> $AFTER"
