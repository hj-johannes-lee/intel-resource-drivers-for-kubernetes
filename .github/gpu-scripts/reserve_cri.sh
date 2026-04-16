#!/usr/bin/env bash

BASE_URL="https://gtax-presi-fm.intel.com/api/v1"
INTEL_USERNAME=${INTEL_USERNAME:-}
INTEL_PASSWORD=${INTEL_PASSWORD:-}
GTA_PASSWORD=${GTA_PASSWORD:-}

# ─── Helper Functions ──────────────────────────────────────────────────────────
# Normalize GTA-X API payload to a non-empty client array.
parse_response() {
  local raw="$1"
  local parsed

  parsed=$(echo "$raw" | jq -c '.data // .') || return 1
  echo "$parsed" | jq -e 'arrays and length > 0' > /dev/null 2>&1 || return 1
  printf '%s\n' "$parsed"
}

ensure_sshpass() {
  if ! command -v sshpass > /dev/null 2>&1; then
    echo "sshpass not found, installing..."
    sudo apt-get install -y sshpass
  fi
}

require_ssh_identity() {
  if [ -f "$HOME/.ssh/id_ed25519.pub" ] || [ -f "$HOME/.ssh/id_rsa.pub" ]; then
    return 0
  fi

  echo "ERROR: No SSH public key found (~/.ssh/id_ed25519.pub or ~/.ssh/id_rsa.pub)."
  return 1
}

# Copy local public key to the reserved host. This must succeed to mark reservation success.
copy_ssh_key() {
  local ip="$1"
  ensure_sshpass || return 1
  require_ssh_identity || return 1

  echo "🔑 Copying SSH public key to gta@${ip}..."
  sshpass -p "$GTA_PASSWORD" ssh-copy-id -o StrictHostKeyChecking=no "gta@${ip}"
}

# Fetch candidate clients for requested status (idle/running) in the reservation window.
fetch_clients() {
  local status="$1"
  local response_raw

  echo "[STEP 1] Searching '${status}' clients with platform Crescent Island..." >&2

  response_raw=$(curl -vk -u "${INTEL_USERNAME}:${INTEL_PASSWORD}" \
    -G "${BASE_URL}/clients" \
    --data-urlencode "csq=('platform' LIKE 'Crescent Island' AND 'os_fullname' LIKE 'Ubuntu 24.04')" \
    --data-urlencode "status=${status}" \
    --data-urlencode "reservation_start_time=${START_TIME}" \
    --data-urlencode "reservation_end_time=${END_TIME}" \
    --data-urlencode "properties=*")

  parse_response "$response_raw"
}

# Try reservations sequentially and stop on first setup success for reservation + ssh.
reserve_clients() {
  local response="$1"
  local client_ids
  local client_id
  local client_name
  local json_body
  local reserve_response
  local reservation_id
  local reserved_system_ip
  local error_msg

  client_ids=$(echo "$response" | jq -r '.[].id')
  if [ -z "$client_ids" ]; then
    echo "No candidate clients found."
    return 1
  fi

  echo "Found client IDs:"
  echo "$client_ids"
  echo "[STEP 2] Creating reservations for each client..."

  for client_id in $client_ids; do
    client_id=$(echo "$client_id" | tr -d '[:space:]\r\n')
    client_name=$(echo "$response" | jq -r --argjson id "$client_id" '.[] | select(.id == $id) | .name')
    echo ""
    echo "📌 Trying reservation for: ${client_name} (ID: ${client_id})"

    json_body=$(jq -n \
      --argjson id "$client_id" \
      --arg owner "hyeongju" \
      --arg start_time "$START_TIME" \
      --arg end_time "$END_TIME" \
      '{
        clients: [{id: $id}],
        owners: [$owner],
        start: $start_time,
        end: $end_time,
        disable_internet_access_management: true
      }')

    reserve_response=$(curl -sk -u "${INTEL_USERNAME}:${INTEL_PASSWORD}" \
      -X POST "${BASE_URL}/reservations" \
      -H "Content-Type: application/json" \
      -d "$json_body")

    echo "$reserve_response"

    reservation_id=$(echo "$reserve_response" | jq -r '.[0].id // empty' 2>/dev/null)
    if [ -n "$reservation_id" ]; then
      reserved_system_ip=$(echo "$reserve_response" | jq -r '.[0].clients[0].properties.client_properties.auto_detected.ip_address')
      echo "✅ Reservation created! Client: ${client_name} (ID: ${client_id}), Reservation ID: ${reservation_id}, IP: ${reserved_system_ip}"
      printf '%s\n' "$reserved_system_ip" > /tmp/reserved_system_ip

      if copy_ssh_key "$reserved_system_ip"; then
        echo "✅ SSH key copied successfully to ${reserved_system_ip}"
        return 0
      fi
      echo "⚠️ SSH key copy failed for ${reserved_system_ip} — trying next client..."
    else
      error_msg=$(echo "$reserve_response" | jq -r '.message // .error // "Unknown error"')
      echo "⚠️ FAILED (${client_name}): ${error_msg} — trying next client..."
    fi
  done

  return 1
}

# One full attempt for a status (idle or running): fetch candidates -> reserve -> configure SSH.
run_reservation_flow() {
  local status="$1"
  local response

  if ! response=$(fetch_clients "$status"); then
    echo "No valid '${status}' client response."
    return 1
  fi

  reserve_clients "$response"
}

# ─── Setup Proxy  ─────────────────────────────────────────────────────────────
export no_proxy="intel.com,*.intel.com,localhost,127.0.0.1"
export NO_PROXY="intel.com,*.intel.com,localhost,127.0.0.1"

# ─── Calculate Reservation Time Window ────────────────────────────────────────
START_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
END_TIME=$(date -u -d "+2 hour" +"%Y-%m-%dT%H:%M:%SZ")
echo "=========================================="
echo " GTA-X Idle CRI Client Reservation Script"
echo "=========================================="
echo "Reservation Start : $START_TIME"
echo "Reservation End   : $END_TIME"
echo ""

# First pass: prefer idle systems.
if ! run_reservation_flow "idle"; then
  echo "Idle flow failed. Retrying full flow with running clients..."
  # Fallback pass: retry from scratch with running systems.
  if ! run_reservation_flow "running"; then
    echo "ERROR: No client could be reserved and configured successfully."
    exit 1
  fi
fi

echo ""
echo "=========================================="
echo " Done!"
echo "=========================================="
exit 0
