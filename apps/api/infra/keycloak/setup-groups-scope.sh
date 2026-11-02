#!/bin/bash
# Adds the "groups" client scope to the local test realm. The API asks for
# the scopes "openid email profile groups" by default (oidc_scopes), and
# Keycloak refuses a sign-in that asks for a scope the client does not have.
# Importing the scope with the realm would replace Keycloak's built-in
# scopes, so it is added once the realm exists. Safe to run more than once.
set -euo pipefail

KCADM=/opt/keycloak/bin/kcadm.sh
SERVER="${KEYCLOAK_URL:-http://keycloak:8080}"

"$KCADM" config credentials --server "$SERVER" --realm master \
  --user "${KC_BOOTSTRAP_ADMIN_USERNAME}" --password "${KC_BOOTSTRAP_ADMIN_PASSWORD}"

if "$KCADM" get client-scopes -r pupitre --fields name --format csv --noquotes | grep -qx groups; then
  echo "groups scope already present"
  exit 0
fi

SCOPE_ID=$("$KCADM" create client-scopes -r pupitre -i \
  -s name=groups -s protocol=openid-connect -s 'attributes."include.in.token.scope"=true')

"$KCADM" create "client-scopes/$SCOPE_ID/protocol-mappers/models" -r pupitre \
  -s name=groups -s protocol=openid-connect -s protocolMapper=oidc-group-membership-mapper \
  -s 'config."claim.name"=groups' -s 'config."full.path"=false' \
  -s 'config."id.token.claim"=true' -s 'config."access.token.claim"=true' -s 'config."userinfo.token.claim"=true'

CLIENT_ID=$("$KCADM" get clients -r pupitre -q clientId=pupitre --fields id --format csv --noquotes)
"$KCADM" update "clients/$CLIENT_ID/default-client-scopes/$SCOPE_ID" -r pupitre

echo "groups scope added"
