#!/usr/bin/python3
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
"""
:mod:`trc_update_test` --- SCION TRC update tests
=======================================================
"""

# Stdlib
import argparse
import logging
import re
import os
import signal
import sys
import time

# SCION
from integration.cert_req_test import TestCertReq, TestCertClient
from lib.crypto.trc import TRC, get_trc_file_path
from lib.defines import GEN_PATH, AS_LIST_FILE
from lib.main import main_wrapper
from lib.packet.cert_mgmt import TRCRequest
from integration.base_cli_srv import (
    setup_main,
)
from lib.util import load_yaml_file, read_file, write_file
from tools.trc_signer import sign_trc
from topology.generator import TopoID


def as_core_list():
    as_dict = load_yaml_file(os.path.join(GEN_PATH, AS_LIST_FILE))
    core_list = []
    for as_str in as_dict.get("Core", []):
        core_list.append(TopoID(as_str))
    return core_list


def update_trcs(cs_list):
    core_ases = as_core_list()
    ases = parse_cs_list(cs_list)
    for i, cs in enumerate(cs_list):
        logging.info("Updating TRC for %s on %s", ases[i].ISD(), cs)
        if ases[i] not in core_ases:
            logging.info("CS is in non-core AS %s", ases[i])
            sys.exit(1)
        conf_dir = get_cs_conf_dir(ases[i])
        old_trc_path, trc_path = generate_new_trc(conf_dir, ases[i])
        for isd_as in core_ases:
            if isd_as[0] == ases[i][0]:
                sign_trc(get_cs_conf_dir(isd_as), trc_path, trc_path)
                logging.info("TRC signed by %s", isd_as)
        trc = TRC.from_raw(read_file(trc_path))
        old_trc = TRC.from_raw(read_file(old_trc_path))
        trc.verify(old_trc)
        output = os.popen('ps ax | grep "cert_server.*%s" | grep -v "grep"' % cs).read()
        pid = int(output.split()[0])
        logging.info("Sending SIGHUP to pid %s", pid)
        os.kill(pid, signal.SIGHUP)


def get_cs_conf_dir(topo_id, cs_name=None):
    cs_name = cs_name or "cs%s-%s" % (topo_id, 1)
    return os.path.join(GEN_PATH, topo_id.ISD(), topo_id.AS(), cs_name)


def generate_new_trc(conf_dir, isd_as):
    old_trc_path = get_trc_file_path(conf_dir, isd_as[0], 0)
    logging.debug("Read TRC from file %s.", old_trc_path)
    trc = TRC.from_raw(read_file(old_trc_path))
    trc.version += 1
    trc.create_time = int(time.time())
    trc.exp_time = trc.create_time + trc.VALIDITY_PERIOD
    trc.grace_period = 18000
    trc_path = get_trc_file_path(conf_dir, isd_as[0], trc.version)
    write_file(trc_path, str(trc))
    logging.debug("Write TRC to file %s.", trc_path)
    return old_trc_path, trc_path


def parse_cs_list(cs_list):
    as_list = []
    for cs in cs_list:
        isd, as_, ind = (int(x) for x in re.findall(r'\d+', cs))
        as_list.append(TopoID.from_values(isd, as_))
    return as_list


class TestUpdatedCertClient(TestCertClient):

    def __init__(self, finished, addr, dst_ia, trc_version):
        self.TRC_VERSION = trc_version
        super().__init__(finished, addr, dst_ia)

    def _create_payload(self, _):
        logging.info("Requesting %sv%s", self.dst_ia, self.TRC_VERSION)
        return TRCRequest.from_values(self.dst_ia, self.TRC_VERSION)

    def _handle_response(self, spkt):
        pld = spkt.parse_payload()
        logging.debug("Got:\n%s", spkt)
        if (self.dst_ia[0], self.TRC_VERSION) == pld.trc.get_isd_ver():
            logging.debug("TRC query success")
            self.success = True
            self.finished.set()
            return True
        logging.error("TRC query failed")
        return False


class TestUpdatedCertReq(TestCertReq):
    NAME = "UpdatedTRCReqTest"

    def __init__(self, client, server, sources, destinations, local=True,
                 max_runs=None, retries=0, trc_version=0):
        self.TRC_VERSION = trc_version
        super().__init__(client, server, sources, destinations, local, max_runs, retries)

    def _create_client(self, finished, addr, dst_ia):
        return TestUpdatedCertClient(finished, addr, dst_ia, self.TRC_VERSION)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--trcupdate', action='store_true',
                        help="Do TRC update")
    parser.add_argument('--cslist', help='List of CS with updated TRC')
    args, srcs, dsts = setup_main("trc_update_test", parser)
    if args.trcupdate:
        update_trcs(args.cslist.split())
        return
    trc_dsts = parse_cs_list(args.cslist.split())
    TestUpdatedCertReq(args.client, args.server, srcs, trc_dsts, max_runs=args.runs,
                       retries=args.retries, trc_version=1).run()

if __name__ == "__main__":
    main_wrapper(main)
