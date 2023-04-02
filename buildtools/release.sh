#!/bin/bash

if [ ! -e .git ]; then
    echo "not in an development environment, no .git directory found" >&2
    exit 1
fi

set -ex

export LC_TIME=C
OLDVERSION="$(./buildtools/get_version | cut -d . -f 1,2)"

NEWVERSION=$(dialog --stdout --inputbox "New Version:" 0 0 "v$OLDVERSION")
NEWVERSION=${NEWVERSION#"v"}

if [ "v$OLDVERSION" = "v$NEWVERSION" -o "x$NEWVERSION" = "x" ]; then
	echo "no changes"
	exit 1
fi

sed -i -e 's/VERSION =.*/VERSION = "'$NEWVERSION'"/g' snclient.go
sed -i Changes -e "s/^next:.*/$(printf "%-10s %s" ${NEWVERSION} "$(date)")/"
git add Changes snclient.go
git commit -vs -m "release v${NEWVERSION}"
git tag -f "v$NEWVERSION"
git show -1
