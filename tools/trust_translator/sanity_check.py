#!/usr/bin/python3
# Copyright 2018 ETH Zurich
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
:mod:`trc_translator` --- Translate outdated TRC to the correct format
===========================================
"""

# Stdlib
import argparse
from collections import defaultdict

from lib.crypto.certificate_chain import CertificateChain
from lib.crypto.trc import TRC
from lib.errors import SCIONVerificationError
from tools.trust_translator.logger import LVLS, Logger

log = Logger()


def load_trcs(files):
    trcs = defaultdict(list)
    for file in files:
        try:
            f = open(file, "r")
        except FileNotFoundError:
            fatal("File '%s' does not exist. Abort" % file)
        trc = TRC.from_raw(f.read())
        f.close()
        try:
            trc._verify_signatures(trc)
        except SCIONVerificationError as e:
            fatal("Invalid signatures for TRC %s. Reason: %s" % (file, e))
        trcs[trc.isd].append((file, trc))
    return trcs


def verify_chains(files, trcs):
    for file in files:
        try:
            f = open(file, "r")
        except FileNotFoundError:
            fatal("File '%s' does not exist. Abort" % file)
        chain = CertificateChain.from_raw(f.read())
        f.close()
        ia, ver = chain.get_leaf_isd_as_ver()
        for trc_file, trc in trcs[ia[0]]:
            try:
                chain.verify(str(ia), trc)
            except SCIONVerificationError as e:
                fatal("Unable to verify chain %s with TRC %s. Reason: %s" % (file, trc_file, e))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-l', '--loglevel', default='INFO',
                        help='Suffix used if mode==SUFFIX (Default: %(default)s)')
    parser.add_argument('-t', '--trcs', nargs="+", required=True, help='List of TRC files')
    parser.add_argument('-c', '--chains', nargs="+", required=True, help='List of Cert Chains')
    args = parser.parse_args()
    if args.loglevel not in LVLS:
        fatal("Invalid loglevel. Available: [%s, %s, %s, %s]" % LVLS)
    log.set_level(args.loglevel)

    verify_chains(args.chains, load_trcs(args.trcs))
    log.info("All chains verifiable")


def fatal(msg):
    log.error(msg)
    exit(-1)

if __name__ == '__main__':
    main()
