#!/usr/bin/with-contenv bashio

export HA_URL="http://supervisor/core/api"
export HA_AUTH_TOKEN="${SUPERVISOR_TOKEN}"

exec /usr/bin/frisi
