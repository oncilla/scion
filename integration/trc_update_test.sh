#!/bin/bash
# Copyright 2016 ETH Zurich
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o pipefail

log() {
    echo "========> ($(date -u --rfc-3339=seconds)) $@"
}

check_cs_exists() {
    find gen/*/*/${cs}* 1>/dev/null 2>&1
    return $?
}

cs_list=""

for cs in "$@"; do
    if ! check_cs_exists "$cs"; then
        log "${cs} does not exist. TRC update test."
        exit 0
    fi
    cs_list="$cs $cs_list"
done

export PYTHONPATH=python/:.
log "Testing connectivity between all the hosts."
python/integration/end2end_test.py -l ERROR
result=$?
if [ ${result} -ne 0 ]; then
    log "E2E test failed. (${result})"
    exit ${result}
fi
# Bring down routers.
SLEEP=10
log "Update TRC and waiting for ${SLEEP}s."
python/integration/trc_update_test.py -l ERROR --trcupdate --cslist "$cs_list"
if [ $? -ne 0 ]; then
    log "Failed TRC update."
    exit 1
fi
sleep ${SLEEP}s
# Check TRCs have been distributed
log "Testing TRC update has reached all ASes (with retries)."
python/integration/trc_update_test.py -l ERROR --cslist "$cs_list"
result=$?
if [ $result -ne 0 ]; then
    log "TRC update hs not reached all ASes. (${result})"
    exit ${result}
fi
log "Testing connectivity between all the hosts."
python/integration/end2end_test.py -l ERROR
result=$?
if [ ${result} -ne 0 ]; then
    log "E2E test failed. (${result})"
    exit ${result}
fi
exit ${result}

