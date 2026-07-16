#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

TESTING_DIR="$1"

mkdir -p "${TESTING_DIR}"

(
    cd "${TESTING_DIR}"

    mkdir -p A

    touch A/file.txt
    for ((i=1; i<=100; i++)); do
        printf 'This is a test line.\n' >> A/file.txt
    done

    mkdir -p B

    ln -s ../B A/toB
    ln -s ../A B/toA

    FILE_COUNT=$(find "${TESTING_DIR}" -type f | wc -l | tr -d ' ')
    echo "ok - Generated ${FILE_COUNT} files for testing"

    REAL_DIRS=$(find "${TESTING_DIR}" -type d | wc -l | tr -d ' ')
    REAL_DIRS=$((REAL_DIRS - 1))

    SYMLINK_DIRS=$(find "${TESTING_DIR}" -type l | while IFS= read -r link; do
        [ -d "${link}" ] && echo 1
    done | wc -l | tr -d ' ')

    DIRECTORY_COUNT=$((REAL_DIRS + SYMLINK_DIRS))
    echo "ok - Generated ${DIRECTORY_COUNT} directories for testing"

    echo "printing the tree of the files"
    if command -v tree >/dev/null 2>&1; then
        tree "${TESTING_DIR}"
    else
        echo "warning: tree command not found, skipping tree output"
    fi
)

exit 0
