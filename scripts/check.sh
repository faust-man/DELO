#!/bin/sh
# Run from any directory; a dedicated PostgreSQL is mandatory.
set -eu
cd "$(dirname "$0")/.."
: "${TEST_DATABASE_URL:?Set TEST_DATABASE_URL to an isolated PostgreSQL database}"
(cd backend && go test -count=1 ./... && go vet ./...)
(cd frontend && npm test && npm run build)
python scripts/check-k8s.py
kubectl create --dry-run=client --validate=false -f infra/k8s/local/demo-admin.yaml -o json > /dev/null
git diff --check
