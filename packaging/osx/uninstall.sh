#!/bin/sh

if [ "$(id -nu)" != "root" ]; then
  exit 1
fi

# stop the agent
echo -n "** stopping the agent..."
launchctl unload /Library/LaunchDaemons/com.snclient.snclient.plist
echo  " OK"

# remove all files
echo -n "** removing files..."
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
echo  " OK"

# remove pkg from pkg database
echo -n "** removing pkg..."
pkgutil --forget com.snclient.snclient
echo  " OK"

# remove this script
rm -rf /usr/local/bin/snclient_uninstall.sh

echo "** SNClient uninstalled successfully"