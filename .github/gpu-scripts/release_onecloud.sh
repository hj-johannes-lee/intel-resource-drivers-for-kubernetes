#!/usr/bin/env bash
set -euo pipefail

system_id="573563"

echo "Using system_id: $system_id"

# Power off the system
api_response=$(curl -s -w "\n%{http_code}" -X 'GET' "https://onecloudapi.intel.com/${CI_USER_ONECLOUD_TOKEN}/system/powercycle/${system_id}/OFF")
http_code=$(echo "$api_response" | tail -n1)
api_content=$(echo "$api_response" | head -n -1)

if [ "$http_code" -ne 200 ]; then
    echo "API request failed with HTTP code $http_code. Response: $api_content" >&2
    echo "Power off using the API was not successful, but the system will still be released automatically in at most 1 hour." >&2
    exit 0
fi

if ! power_off_result=$(echo "$api_content" | jq -r '.result' 2>/dev/null); then
    echo "Failed to parse API response: $api_content" >&2
    echo "Power off using the API was not successful, but the system will still be released automatically in at most 1 hour." >&2
    exit 0
fi

if [ "$power_off_result" != "done" ]; then
    echo "Power off using the API was not successful, but the system will still be released automatically in at most 1 hour." >&2
    exit 0
fi

echo "The system is OFF"


# Release reservation
file_path_reservation_id="$(dirname "$0")/reservation_id"

if [ ! -f "$file_path_reservation_id" ]; then
    echo "No reservation_id file found. It may have been already released.
Even if not, the system will still be released automatically in at most 1 hour." >&2
    exit 0
fi

reservation_id=$(cat "$file_path_reservation_id")
api_response=$(curl -s -w "\n%{http_code}" -X 'DELETE' "https://onecloudapi.intel.com/${CI_USER_ONECLOUD_TOKEN}/reservation/${reservation_id}")
http_code=$(echo "$api_response" | tail -n1)
api_content=$(echo "$api_response" | head -n -1)

if [ "$http_code" -ne 200 ]; then
    echo "API request failed with HTTP code $http_code. Response: $api_content" >&2
    echo "Delete using the API was not successful, but the system will still be released automatically in at most 1 hour." >&2
    exit 0
fi

if ! delete_result=$(echo "$api_content" | jq -r '.result' 2>/dev/null); then
    echo "Failed to parse API response: $api_content" >&2
    echo "Delete using the API was not successful, but the system will still be released automatically in at most 1 hour." >&2
fi

if [ "$delete_result" == "null" ]; then
    echo "Delete using the API was not successful, but the system will still be released automatically in at most 1 hour." >&2
    exit 0
fi

echo "The system has been released."
rm -f "$file_path_reservation_id"
