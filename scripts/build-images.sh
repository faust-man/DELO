#!/bin/sh
# Build/push independently versioned Linux images; deployment uses kubectl directly.
set -eu
prefix=${IMAGE_PREFIX:-delo}
tag=${IMAGE_TAG:-micro}
if [ "$#" -eq 0 ]; then
 set -- identity contacts organizations education bookings subscriptions finance artworks inventory exhibitions reports audit notifications gateway worker admin web
fi
for component do
 case "$component" in
  identity|contacts|organizations|education|bookings|subscriptions|finance|artworks|inventory|exhibitions|reports|audit|notifications|gateway|worker)
   docker build --platform "${PLATFORM:-linux/amd64}" --target service --build-arg "SERVICE=$component" -t "$prefix-$component:$tag" backend ;;
  admin)
   docker build --platform "${PLATFORM:-linux/amd64}" --target admin -t "$prefix-admin:$tag" backend ;;
  web)
   docker build --platform "${PLATFORM:-linux/amd64}" -t "$prefix-web:$tag" frontend ;;
  *) printf 'Unknown service: %s\n' "$component" >&2; exit 1 ;;
 esac
 if [ "${PUSH:-false}" = true ]; then docker push "$prefix-$component:$tag"; fi
done
