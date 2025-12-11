#!/bin/bash

# This script is intended to generate files to be tested for check_files
# These files need to be generated dynamically, so they cant be simply saved in the repository.

set -o errexit
set -o nounset
set -o pipefail

TESTING_DIR="$1"

mkdir -p ${TESTING_DIR}

# Generate it with local timezone
TODAY=$(date -d 'today 00:00:00' +%F)

(
    cd ${TESTING_DIR}

    mkdir -p ${TESTING_DIR}

    touch ${TESTING_DIR}/file1.txt
    touch ${TESTING_DIR}/file2
    mkdir -p ${TESTING_DIR}/directory1
    touch ${TESTING_DIR}/directory1/directory1-file3.txt
    touch ${TESTING_DIR}/directory1/directory1-file4
    mkdir -p ${TESTING_DIR}/directory1/directory1-directory2
    touch ${TESTING_DIR}/directory1/directory1-directory2/directory1-directory2-file5
    touch ${TESTING_DIR}/directory1/directory1-directory2/directory1-directory2-file6
    touch ${TESTING_DIR}/directory1/directory1-directory2/directory1-directory2-file7
    mkdir -p ${TESTING_DIR}/directory1/directory1-directory2/directory1-directory2-directory3
    touch ${TESTING_DIR}/directory1/directory1-directory2/directory1-directory2-directory3/directory1-directory2-directory3-file8
    mkdir -p ${TESTING_DIR}/directory4
    touch ${TESTING_DIR}/directory4/directory4-file9.exe
    touch ${TESTING_DIR}/directory4/directory4-file10.html
    mkdir -p ${TESTING_DIR}/directory4/directory4-directory5
    mkdir -p ${TESTING_DIR}/directory4/directory4-directory6
    touch ${TESTING_DIR}/directory4/directory4-directory5/directory4-directory5-file11

    FILE_COUNT=$(find ${TESTING_DIR} -type f -printf "." | wc -c)
    echo "ok - Generated ${FILE_COUNT} files for testing"

    # This also counts the TESTING_DIR
    DIRECTORY_COUNT=$(find ${TESTING_DIR} -type d -printf "." | wc -c)
    #echo "DIRECTORY_COUNT=${DIRECTORY_COUNT}"
    echo "ok - Generated $(( DIRECTORY_COUNT - 1 )) directories for testing"

    echo "printing the tree of the files"
    tree ${TESTING_DIR}
)

exit 0
