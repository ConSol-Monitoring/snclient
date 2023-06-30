#!/usr/bin/env bash

state=$1
if [ -n "$2" ]; then
  message=": $2"
else
  message=""
fi

if [[ "$state" != "0" && "$state" != "1" && "$state" != "2" && "$state" != "3" ]]; then
    echo "Invalid state argument. Please provide one of: 0, 1, 2, 3"
    exit 3
fi

if [ "$state" = "0" ]; then
    echo "OK$message"
    exit_status=0
elif [ "$state" = "1" ]; then
    echo "WARNING$message"
    exit_status=1
elif [ "$state" = "2" ]; then
    echo "CRITICAL$message"
    exit_status=2
elif [ "$state" = "3" ]; then
    echo "UNKNOWN$message"
    exit_status=3
fi

exit $exit_status

