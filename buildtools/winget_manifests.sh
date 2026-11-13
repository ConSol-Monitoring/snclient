#!/bin/sh

# Generate WinGet manifests from the templates.
# The Makefile uses this script.
# The PowerShell counterpart is buildtools/winget_manifests.ps1.
# Keep both scripts in sync.
# TAG_VERSION is the release tag without the leading "v".
# PACKAGE_VERSION is the MSI ProductVersion and ARP DisplayVersion.
# WinGet PackageVersion must match the installed MSI version.
# Installer URLs use TAG_VERSION.
# PackageVersion uses PACKAGE_VERSION.

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(dirname -- "$SCRIPT_DIR")
TEMPLATES=${WINGET_TEMPLATES:-$ROOT/packaging/winget/templates}

PACKAGE_VERSION=${WINGET_PACKAGE_VERSION:-}
TAG_VERSION=${WINGET_TAG_VERSION:-}
RELEASE_DATE=${WINGET_RELEASE_DATE:-}
OUTDIR=${WINGET_OUTDIR:-}

if [ -z "$PACKAGE_VERSION" ]; then
    PACKAGE_VERSION=$("$SCRIPT_DIR/get_winget_version")
fi
if [ -z "$TAG_VERSION" ]; then
    TAG_VERSION=$("$SCRIPT_DIR/get_version")
fi
if [ -z "$RELEASE_DATE" ]; then
    RELEASE_DATE=$(date -u +%Y-%m-%d)
fi
if [ -z "$OUTDIR" ]; then
    OUTDIR="$ROOT/packaging/winget/$PACKAGE_VERSION"
fi

SHA256_X64=${WINGET_SHA256_X64:-}
SHA256_X86=${WINGET_SHA256_X86:-}
SHA256_ARM64=${WINGET_SHA256_ARM64:-}
PRODUCT_CODE_X64=${WINGET_PRODUCTCODE_X64:-}
PRODUCT_CODE_X86=${WINGET_PRODUCTCODE_X86:-}
PRODUCT_CODE_ARM64=${WINGET_PRODUCTCODE_ARM64:-}

require_value() {
    name=$1
    value=$2
    if [ -z "$value" ]; then
        echo "missing $name" >&2
        exit 1
    fi
}

require_hash() {
    name=$1
    value=$2
    if [ "${#value}" -ne 64 ]; then
        echo "$name must be a 64 character SHA256 hash" >&2
        exit 1
    fi
    case "$value" in
        *[!0-9A-Fa-f]*)
            echo "$name must contain only hexadecimal characters" >&2
            exit 1
            ;;
    esac
}

require_product_code() {
    name=$1
    value=$2
    case "$value" in
        '{'*'}') ;;
        *)
            echo "$name must be a braced MSI ProductCode" >&2
            exit 1
            ;;
    esac
}

require_value WINGET_SHA256_X64 "$SHA256_X64"
require_value WINGET_SHA256_X86 "$SHA256_X86"
require_value WINGET_SHA256_ARM64 "$SHA256_ARM64"
require_value WINGET_PRODUCTCODE_X64 "$PRODUCT_CODE_X64"
require_value WINGET_PRODUCTCODE_X86 "$PRODUCT_CODE_X86"
require_value WINGET_PRODUCTCODE_ARM64 "$PRODUCT_CODE_ARM64"

require_hash WINGET_SHA256_X64 "$SHA256_X64"
require_hash WINGET_SHA256_X86 "$SHA256_X86"
require_hash WINGET_SHA256_ARM64 "$SHA256_ARM64"
require_product_code WINGET_PRODUCTCODE_X64 "$PRODUCT_CODE_X64"
require_product_code WINGET_PRODUCTCODE_X86 "$PRODUCT_CODE_X86"
require_product_code WINGET_PRODUCTCODE_ARM64 "$PRODUCT_CODE_ARM64"

case "$PACKAGE_VERSION" in
    [0-9]*.[0-9]*.[0-9]*) ;;
    *)
        echo "invalid package version: $PACKAGE_VERSION" >&2
        exit 1
        ;;
esac
case "$TAG_VERSION" in
    [0-9]*.[0-9]*) ;;
    *)
        echo "invalid tag version: $TAG_VERSION" >&2
        exit 1
        ;;
esac

mkdir -p "$OUTDIR"

for template in "$TEMPLATES"/*.yaml.in; do
    if [ ! -e "$template" ]; then
        echo "no templates found in $TEMPLATES" >&2
        exit 1
    fi
    output="$OUTDIR/$(basename "$template" .in)"
    sed \
        -e "s|__PACKAGE_VERSION__|$PACKAGE_VERSION|g" \
        -e "s|__TAG_VERSION__|$TAG_VERSION|g" \
        -e "s|__RELEASE_DATE__|$RELEASE_DATE|g" \
        -e "s|__SHA256_X64__|$SHA256_X64|g" \
        -e "s|__SHA256_X86__|$SHA256_X86|g" \
        -e "s|__SHA256_ARM64__|$SHA256_ARM64|g" \
        -e "s|__PRODUCT_CODE_X64__|$PRODUCT_CODE_X64|g" \
        -e "s|__PRODUCT_CODE_X86__|$PRODUCT_CODE_X86|g" \
        -e "s|__PRODUCT_CODE_ARM64__|$PRODUCT_CODE_ARM64|g" \
        "$template" > "$output"
    if grep -q '__[A-Z0-9_]*__' "$output"; then
        echo "unresolved placeholder in $output" >&2
        exit 1
    fi
    echo "generated $output"
done
