#!/bin/sh
# Signs Windows programs with Azure Trusted Signing via jsign.
#
#   packaging/windows/sign.sh FILE...
#
# Needs AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, TRUSTED_SIGNING_ENDPOINT (e.g.
# weu.codesigning.azure.net), TRUSTED_SIGNING_ACCOUNT, TRUSTED_SIGNING_PROFILE, and JSIGN (jsign's jar).
# Without the secret the files stay unsigned, and SmartScreen warns about them.
set -e
if [ -z "$AZURE_CLIENT_SECRET" ]; then
    echo "No Windows signing set up: $* stay unsigned."
    exit 0
fi

token=$(curl -fsS "https://login.microsoftonline.com/$AZURE_TENANT_ID/oauth2/v2.0/token" \
    --data grant_type=client_credentials \
    --data-urlencode "client_id=$AZURE_CLIENT_ID" \
    --data-urlencode "client_secret=$AZURE_CLIENT_SECRET" \
    --data-urlencode scope=https://codesigning.azure.net/.default | jq -r '.access_token // empty')
if [ -z "$token" ]; then
    echo "Azure gave no token: check AZURE_TENANT_ID, AZURE_CLIENT_ID and AZURE_CLIENT_SECRET." >&2
    exit 1
fi
[ -n "$GITHUB_ACTIONS" ] && echo "::add-mask::$token"

java -jar "${JSIGN:?JSIGN is the path to the jsign jar}" \
    --storetype TRUSTEDSIGNING --keystore "$TRUSTED_SIGNING_ENDPOINT" --storepass "$token" \
    --alias "$TRUSTED_SIGNING_ACCOUNT/$TRUSTED_SIGNING_PROFILE" \
    --tsaurl http://timestamp.acs.microsoft.com --tsmode RFC3161 --replace "$@"
