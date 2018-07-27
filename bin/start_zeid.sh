#!/bin/sh

API_KEY=$(snapctl get api-key)
API_SECRET=$(snapctl get api-secret)
SERIAL_NUMBER=$(snapctl get serial-number)

exec $SNAP/bin/zeid -api-key $API_KEY -api-secret $API_SECRET -serial-number $SERIAL_NUMBER
