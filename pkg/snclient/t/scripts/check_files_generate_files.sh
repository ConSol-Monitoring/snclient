#!/bin/bash

# This script is intended to generate files to be tested for check_files
# These files need to be generated dynamically, so they cant be simply saved in the repository.

set -o errexit
set -o nounset
set -o pipefail

TESTING_DIR="$1"

mkdir -p ${TESTING_DIR}

TODAY=$(date -d 'today 00:00:00' +%F)

echo "${TODAY} ${TESTING_DIR}" > /tmp/check_files_generate_files

(
    cd ${TESTING_DIR}

    touch --time modify --date $(date -d "${TODAY} - 1 years" +%FT%T) one_year_ago
    touch --time modify --date $(date -d "${TODAY} - 1 months " +%FT%T) one_month_ago
    touch --time modify --date $(date -d "${TODAY} - 1 week" +%FT%T) one_week_ago
    touch --time modify --date $(date -d "${TODAY} - 2 days" +%FT%T) two_days_ago
    touch --time modify --date $(date -d "${TODAY} - 1 days" +%FT%T) yesterday
    touch --time modify --date $(date -d "${TODAY}" +%FT%T) today
    touch --time modify --date $(date -d "${TODAY} + 1 days" +%FT%T) tomorrow
    touch --time modify --date $(date -d "${TODAY} + 2 days" +%FT%T) two_days_from_now_on
    touch --time modify --date $(date -d "${TODAY} + 1 weeks" +%FT%T) one_week_from_now_on
    touch --time modify --date $(date -d "${TODAY} + 1 month" +%FT%T) one_month_from_now_on
    touch --time modify --date $(date -d "${TODAY} + 1 years" +%FT%T) one_year_from_now_on

    FILE_COUNT=$(find ${TESTING_DIR} -type f -printf "." | wc -c)

    echo "ok - Generated ${FILE_COUNT} files for testing"
)

exit 0

