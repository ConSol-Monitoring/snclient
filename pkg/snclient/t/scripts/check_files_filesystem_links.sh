#!/bin/bash

# This script is intended to generate systemlink files to be used in a test
# These files need to be generated dynamically, so they cant be simply saved in the repository.

set -o errexit
set -o nounset
set -o pipefail

TESTING_DIR="$1"

mkdir -p "${TESTING_DIR}"

(
    cd "${TESTING_DIR}"

    mkdir -p "dir1"

    touch "dir1/file1.txt"
    echo > "dir1/file1.txt"

    for ((i=1; i<=100; i++)); do
        printf 'This is a test line \n' >> "dir1/file1.txt"
    done

    # Symbolic link to folder
    ln --symbolic "dir1" "dir1_symbolic1"

    # Symbolic link to file
    ln --symbolic "dir1/file1.txt" "file1_symbolic1.txt"

    # Relative link to folder
    ln --symbolic --relative "dir1" "dir1_relative1"

    # Relative link to file
    ln --symbolic --relative "dir1/file1.txt" "file1_relative1.txt"

    # Physical links to directories are not allowed
    # ln --physical "dir1" "dir1_physical1"

    # Physical link to file
    ln --physical "dir1/file1.txt" "file1_physical1.txt"

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
