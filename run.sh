#!/usr/bin/with-contenv bashio

export HA_URL="http://homeassistant:8123"
export HA_AUTH_TOKEN="${SUPERVISOR_TOKEN}"

exec /usr/bin/frisi
