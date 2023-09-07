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

sed -i -e 's/VERSION =.*/VERSION = "'$NEWVERSION'"/g' pkg/snclient/snclient.go
VERSIONHEADER=$(printf "%-10s %s" ${NEWVERSION} "$(date)")
if grep "^next:" Changes >/dev/null 2>&1; then
	# replace next: with version
	sed -i Changes -e "s/^next:.*/${VERSIONHEADER}/"
else
	# no next: entry found, replace second line with new entry
	sed -i Changes -e "2s/^/\n${VERSIONHEADER}\n         - ...\n/"
fi
git add Changes pkg/snclient/snclient.go
git commit -vs -m "release v${NEWVERSION}"
git tag -f "v$NEWVERSION"
git show -1
