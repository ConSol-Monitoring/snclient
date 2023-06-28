#!/usr/bin/env bash

OK=0
WARNING=1
CRITICAL=2
UNKNOWN=3

state=$1
message=$2

if [[ "$state" != "OK" && "$state" != "Warning" && "$state" != "Critical" && "$state" != "Unknown" ]]; then
    echo "Invalid state argument. Please provide one of: OK, Warning, Critical, Unknown"
    exit $UNKNOWN
fi

echo "$state: $message"
exit_status=$OK

if [ "$state" = "Warning" ]; then
    exit_status=$WARNING
elif [ "$state" = "Critical" ]; then
    exit_status=$CRITICAL
elif [ "$state" = "Unknown" ]; then
    exit_status=$UNKNOWN
fi

exit $exit_status

