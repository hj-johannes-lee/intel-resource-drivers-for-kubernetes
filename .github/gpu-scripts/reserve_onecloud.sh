#!/usr/bin/env bash
set -euo pipefail

system_id="573563"
os_image="${os_image:-4042004}"

reservation_id=$(curl -s -X 'POST' \
    'https://onecloudapi.intel.com/reservation' \
    -H 'accept: application/json' \
    -H 'Content-Type: application/json' \
    -d '{
          "systemid": "'"${system_id}"'",
          "osimageid": "'"${os_image}"'",
          "hours": "1",
          "token": "'"${CI_USER_ONECLOUD_TOKEN}"'"
        }' | jq -r '.reservationid')

if [ "$reservation_id" != "null" ]; then
    echo "$reservation_id" > "$(dirname "$0")/reservation_id"
    echo "$system_id" > "$(dirname "$0")/system_id"
    echo "✅ System ($system_id) reserved successfully. Reservation ID: $reservation_id"
else
    echo "❌ Failed to reserve system $system_id" >&2
    exit 1
fi
