#!/bin/bash

PACKAGE_NAME="snclient"
PACKAGE_VERSION="$(cd ../.. && ./buildtools/get_version)"
DAEMON_NAME="com.snclient.snclient"
PACKAGE_DIR="$PACKAGE_NAME-$PACKAGE_VERSION"

mkdir -p "$PACKAGE_DIR/Library/LaunchDaemons"
cp $DAEMON_NAME.plist "$PACKAGE_DIR/Library/LaunchDaemons/"

mkdir -p "$PACKAGE_DIR/usr/local/bin"
cp ../../snclient $PACKAGE_DIR/usr/local/bin/

mkdir -p "$PACKAGE_DIR/etc/snclient"
cp ../snclient.ini $PACKAGE_DIR/etc/snclient/

pkgbuild --root "$PACKAGE_DIR" \
         --identifier $DAEMON_NAME \
         --version $PACKAGE_VERSION \
         --install-location / \
         --scripts . \
         "$PACKAGE_DIR.pkg"

rm -rf "$PACKAGE_DIR"
