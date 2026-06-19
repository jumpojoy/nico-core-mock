#!/bin/bash
# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Post-deployment configuration for local/dev NICo REST installs.
# Grants org admin realm roles to the nico-api Keycloak service account and
# enables imageBasedOperatingSystem on existing sites.
#
# Run after NICo REST and Keycloak are deployed (e.g. after site creation).
#
# Example (VM with external IP):
#   KEYCLOAK_URL=http://10.200.0.1:8082 API_URL=http://10.200.0.1:8388 \
#     bash scripts/post-deployment-config.sh

set -euo pipefail

NAMESPACE="${NAMESPACE:-nico-rest}"
KEYCLOAK_DEPLOYMENT="${KEYCLOAK_DEPLOYMENT:-keycloak}"
KEYCLOAK_REALM="${KEYCLOAK_REALM:-nico-dev}"
KEYCLOAK_ADMIN="${KEYCLOAK_ADMIN:-admin}"
KEYCLOAK_ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin}"
KEYCLOAK_CLIENT_ID="${KEYCLOAK_CLIENT_ID:-nico-api}"
KEYCLOAK_URL="${KEYCLOAK_URL:-http://10.200.0.1:8082}"
API_URL="${API_URL:-http://10.200.0.1:8388}"
ORG="${ORG:-test-org}"
FAKE_INSTANCE_TYPE_NAME="${FAKE_INSTANCE_TYPE_NAME:-mock-x1.large}"
FAKE_ALLOCATION_NAME="${FAKE_ALLOCATION_NAME:-mock-allocation}"
FAKE_ALLOCATION_MACHINES="${FAKE_ALLOCATION_MACHINES:-1}"

SERVICE_ACCOUNT_USERNAME="${SERVICE_ACCOUNT_USERNAME:-service-account-${KEYCLOAK_CLIENT_ID}}"

if [ -z "${KEYCLOAK_CLIENT_SECRET:-}" ]; then
    KEYCLOAK_CLIENT_SECRET=$(kubectl -n "$NAMESPACE" get secret keycloak-client-secret \
        -o jsonpath='{.data.keycloak-client-secret}' 2>/dev/null | base64 -d 2>/dev/null || true)
fi
KEYCLOAK_CLIENT_SECRET="${KEYCLOAK_CLIENT_SECRET:-nico-local-secret}"

kc_exec() {
    kubectl -n "$NAMESPACE" exec "deploy/${KEYCLOAK_DEPLOYMENT}" -- \
        /opt/keycloak/bin/kcadm.sh "$@"
}

wait_for_keycloak() {
    echo "Waiting for Keycloak deployment..."
    kubectl -n "$NAMESPACE" rollout status "deployment/${KEYCLOAK_DEPLOYMENT}" --timeout=240s

    echo "Waiting for Keycloak realm ${KEYCLOAK_REALM}..."
    for i in $(seq 1 240); do
        if curl -sf "${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}" > /dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    echo "ERROR: Keycloak realm ${KEYCLOAK_REALM} not ready at ${KEYCLOAK_URL}" >&2
    exit 1
}

configure_kcadm() {
    echo "Configuring Keycloak admin credentials..."
    kc_exec config credentials \
        --server http://localhost:8080 \
        --realm master \
        --user "$KEYCLOAK_ADMIN" \
        --password "$KEYCLOAK_ADMIN_PASSWORD"
}

role_assigned() {
    local role=$1
    kc_exec get-roles -r "$KEYCLOAK_REALM" --uusername "$SERVICE_ACCOUNT_USERNAME" 2>/dev/null \
        | grep -Fq "$role"
}

assign_realm_role() {
    local role=$1
    if role_assigned "$role"; then
        echo "Role ${role} already assigned to ${SERVICE_ACCOUNT_USERNAME}, skipping"
        return 0
    fi

    echo "Assigning role ${role} to ${SERVICE_ACCOUNT_USERNAME}..."
    kc_exec add-roles -r "$KEYCLOAK_REALM" \
        --uusername "$SERVICE_ACCOUNT_USERNAME" \
        --rolename "$role"
}

assign_service_account_roles() {
    assign_realm_role "${ORG}:PROVIDER_ADMIN"
    assign_realm_role "${ORG}:TENANT_ADMIN"
}

get_service_account_token() {
    local token
    token=$(curl -sf -X POST "${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}/protocol/openid-connect/token" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "grant_type=client_credentials" \
        -d "client_id=${KEYCLOAK_CLIENT_ID}" \
        -d "client_secret=${KEYCLOAK_CLIENT_SECRET}" | jq -r .access_token)

    if [ -z "$token" ] || [ "$token" = "null" ]; then
        echo "ERROR: Failed to acquire service account token" >&2
        exit 1
    fi
    echo "$token"
}

verify_service_account() {
    local token=$1
    echo "Verifying service account token and API access..."

    local response
    response=$(curl -sf -H "Authorization: Bearer ${token}" \
        "${API_URL}/v2/org/${ORG}/nico/service-account/current")

    echo "Service account current:"
    echo "$response" | jq .
}

get_first_site_id() {
    local token=$1
    curl -sf -H "Authorization: Bearer ${token}" \
        "${API_URL}/v2/org/${ORG}/nico/site" | jq -r '.[0].id // empty'
}

enable_site_image_based_os() {
    local token=$1
    echo "Enabling imageBasedOperatingSystem site capability..."

    local site_ids
    site_ids=$(curl -sf -H "Authorization: Bearer ${token}" \
        "${API_URL}/v2/org/${ORG}/nico/site" | jq -r '.[].id // empty')

    if [ -z "$site_ids" ]; then
        echo "No sites found, skipping imageBasedOperatingSystem capability update"
        return 0
    fi

    while IFS= read -r site_id; do
        [ -z "$site_id" ] && continue
        echo "Patching site ${site_id}..."
        local response
        response=$(curl -sf -X PATCH \
            -H "Authorization: Bearer ${token}" \
            -H "Content-Type: application/json" \
            "${API_URL}/v2/org/${ORG}/nico/site/${site_id}" \
            -d '{"capabilities": {"imageBasedOperatingSystem": true}}')
        echo "Site ${site_id} capabilities:"
        echo "$response" | jq '.capabilities'
    done <<< "$site_ids"
}

create_fake_ipblock_and_allocation() {
    local token=$1
    echo "Creating fake IP Block and Allocation for current tenant..."

    local tenant_id
    tenant_id=$(curl -sf -H "Authorization: Bearer ${token}" \
        "${API_URL}/v2/org/${ORG}/nico/tenant/current" | jq -r '.id // empty')
    if [ -z "$tenant_id" ] || [ "$tenant_id" = "null" ]; then
        echo "ERROR: Failed to resolve current tenant ID" >&2
        return 1
    fi
    echo "Current tenant: ${tenant_id}"

    local site_id
    site_id=$(get_first_site_id "$token")

    if [ -z "$site_id" ] || [ "$site_id" = "null" ]; then
        echo "No sites found, skipping IP Block/Allocation creation"
        return 0
    fi
    echo "Using site: ${site_id}"

    local ipblock_id
    ipblock_id=$(curl -sf -H "Authorization: Bearer ${token}" \
        "${API_URL}/v2/org/${ORG}/nico/ipblock?siteId=${site_id}" \
        | jq -r '[.[] | select(.name == "fake-ipblock")][0].id // empty')

    if [ -z "$ipblock_id" ]; then
        echo "Creating fake IP Block 3.3.3.3/32 on site ${site_id}..."
        ipblock_id=$(curl -sf -X POST \
            -H "Authorization: Bearer ${token}" \
            -H "Content-Type: application/json" \
            "${API_URL}/v2/org/${ORG}/nico/ipblock" \
            -d "{
                \"name\": \"fake-ipblock\",
                \"description\": \"Fake IP block for local/dev testing\",
                \"siteId\": \"${site_id}\",
                \"routingType\": \"DatacenterOnly\",
                \"prefix\": \"3.3.3.3\",
                \"prefixLength\": 32,
                \"protocolVersion\": \"IPv4\"
            }" | jq -r '.id // empty')
    else
        echo "IP Block already exists for site ${site_id}: ${ipblock_id}"
    fi

    if [ -z "$ipblock_id" ] || [ "$ipblock_id" = "null" ]; then
        echo "ERROR: Failed to create or find IP Block for site ${site_id}" >&2
        return 1
    fi

    local alloc_id
    alloc_id=$(curl -sf -H "Authorization: Bearer ${token}" \
        "${API_URL}/v2/org/${ORG}/nico/allocation?siteId=${site_id}&tenantId=${tenant_id}&resourceType=IPBlock&resourceTypeId=${ipblock_id}" \
        | jq -r '.[0].id // empty')

    if [ -z "$alloc_id" ] || [ "$alloc_id" = "null" ]; then
        echo "Creating Allocation for tenant ${tenant_id} on site ${site_id}..."
        curl -sf -X POST \
            -H "Authorization: Bearer ${token}" \
            -H "Content-Type: application/json" \
            "${API_URL}/v2/org/${ORG}/nico/allocation" \
            -d "{
                \"name\": \"fake-ipblock-allocation\",
                \"description\": \"Fake IP block allocation for local/dev testing\",
                \"tenantId\": \"${tenant_id}\",
                \"siteId\": \"${site_id}\",
                \"allocationConstraints\": [{
                    \"resourceType\": \"IPBlock\",
                    \"resourceTypeId\": \"${ipblock_id}\",
                    \"constraintType\": \"Reserved\",
                    \"constraintValue\": 32
                }]
            }" | jq '{id, status, allocationConstraints}'
    else
        echo "Allocation already exists for site ${site_id}: ${alloc_id}"
    fi
}

main() {
    echo "Running post-deployment configuration..."
    wait_for_keycloak
    configure_kcadm
    assign_service_account_roles

    local token
    token=$(get_service_account_token)
    verify_service_account "$token"
    enable_site_image_based_os "$token"
    create_fake_ipblock_and_allocation "$token"

    echo "Post-deployment configuration complete."
}

main "$@"
