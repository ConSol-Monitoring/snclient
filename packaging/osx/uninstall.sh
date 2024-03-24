#!/bin/sh

if [ "$(id -nu)" != "root" ]; then
  exit 1
fi

# stop the agent
launchctl unload /Library/LaunchDaemons/com.snclient.snclient.plist

# remove all files
rm -rf /etc/snclient/snclient.ini
rm -rf /etc/snclient/cacert.pem
rm -rf /etc/snclient/server.crt
rm -rf /etc/snclient/server.key
rmdir /etc/snclient >/dev/null 2>&1
rm -rf /var/log/snclient
rm -rf /usr/local/bin/node_exporter
rm -rf /usr/local/bin/snclient
rm -rf /usr/local/share/man/man*/snclient.*
rm -rf /Library/LaunchDaemons/com.snclient.snclient.plist

# remove pkg from pkg database
pkgutil --forget com.snclient.snclient

# remove this script
rm -rf /usr/local/bin/snclient_uninstall.sh
