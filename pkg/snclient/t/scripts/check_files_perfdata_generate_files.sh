#!/bin/bash

# This script is intended to generate files to be tested for check_files
# These files need to be generated dynamically, so they cant be simply saved in the repository.

set -o errexit
set -o nounset
set -o pipefail

TESTING_DIR="$1"

mkdir -p "${TESTING_DIR}"

(
    cd "${TESTING_DIR}"

    # Create files of 512KB (0.5MB)
    dd if=/dev/zero of="${TESTING_DIR}/file_512kb_1.root" bs=1024 count=512 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/file_512kb_2.root" bs=1024 count=512 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/file_512kb_3.root" bs=1024 count=512 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/file_512kb_4.root" bs=1024 count=512 2>/dev/null

    # Create files of 1024KB (1MB)
    dd if=/dev/zero of="${TESTING_DIR}/file_1024kb_1.root" bs=1024 count=1024 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/file_1024kb_2.root" bs=1024 count=1024 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/file_1024kb_3.root" bs=1024 count=1024 2>/dev/null

    mkdir -p "${TESTING_DIR}/a"
    dd if=/dev/zero of="${TESTING_DIR}/a/file_1024kb_1.a" bs=1024 count=1024 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/a/file_1024kb_2.a" bs=1024 count=1024 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/a/file_1024kb_3.a" bs=1024 count=1024 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/a/file_1024kb_4.a" bs=1024 count=1024 2>/dev/null

    mkdir -p "${TESTING_DIR}/b"
    dd if=/dev/zero of="${TESTING_DIR}/b/file_1024kb_1.b" bs=1024 count=1024 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/b/file_1024kb_2.b" bs=1024 count=1024 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/b/file_1024kb_3.b" bs=1024 count=1024 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/b/file_1024kb_4.b" bs=1024 count=1024 2>/dev/null
    dd if=/dev/zero of="${TESTING_DIR}/b/file_1024kb_5.b" bs=1024 count=1024 2>/dev/null

    FILE_COUNT=$(find "${TESTING_DIR}" -type f -printf "." | wc -c)
    echo "ok - Generated ${FILE_COUNT} files for testing"

    DIRECTORY_COUNT=$(find "${TESTING_DIR}" -type d -printf "." | wc -c)
    echo "ok - Generated $(( DIRECTORY_COUNT - 1 )) directories for testing"

    if command -v tree >/dev/null 2>&1; then
        echo "printing the tree of the files"
        tree "${TESTING_DIR}"
    else
        echo "warning: tree command not found, skipping tree output"
    fi
)

exit 0