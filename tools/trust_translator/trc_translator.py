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
import base64
import os

import tools.trust_translator.old_ver.old_trc as old
import lib.crypto.trc as new
from lib.crypto.trc import TRC
from lib.crypto.util import get_online_key_file_path
from lib.defines import TOPO_FILE
from lib.errors import SCIONIOError
from lib.topology import Topology
from lib.util import read_file, write_file
from tools.trust_translator.logger import LVLS, Logger
from tools.trust_translator.old_ver.old_trc import TRC as OldTRC

OUT = "OUTDIR"
OVR = "OVERWRITE"
SUF = "SUFFIX"
log = Logger()


def d_to_s(d):
    return d * 24 * 60 * 60


def load_keys(conf_dirs):
    """
    Load online keys from provided conf dirs. Ignores non-core AS configs.
    Config is required to have a topology file and the online key in the new format.

    :param list(str) conf_dirs: List of conf dir of core ASes
    :return: mapping ISD-AS -> online key
    :rtype: dict
    """
    keys = {}
    for conf_dir in conf_dirs:
        topo = Topology.from_file(os.path.join(conf_dir, TOPO_FILE))
        if not topo.is_core_as:
            log.debug("Provided conf is not from a core AS.")
            continue
        try:
            keys[str(topo.isd_as)] = base64.b64decode(read_file(get_online_key_file_path(conf_dir)))
        except SCIONIOError as e:
            fatal("%s.\nMake sure to confdir has already been translated" % str(e))
    return keys


def update_trcs(files, val_period, keys):
    """
    Load TRCs in old format from files, translate them to the new format and sign them.
    The validity period of all TRCs is set to val_period.

    :param list(str) files: List of TRC file paths to be translated
    :param int val_period: TRC validity period in days
    :param dict keys: Mapping ISD-AS -> online key containing all necessary keys
    :return: List of (orig path, new TRC)-tuples
    :rtype: list((str, TRC))
    """
    trcs = []
    for file in files:
        trc = translate_trc(file, val_period)
        trcs.append((file, sign_trc(trc, keys)))
    return trcs


def translate_trc(file, val_period):
    """
    Load TRC from file and set validity period to val_period.

    :param str file: TRC file path
    :param int val_period: TRC validity period in days
    :return: Translated TRC
    :rtype: TRC
    """
    try:
        f = open(file, "r")
    except FileNotFoundError:
        fatal("File '%s' does not exist. Abort" % file)
    old_trc = OldTRC.from_raw(f.read())
    f.close()
    old_dict = old_trc.dict(True)

    core_dict = {}
    for ia, d in old_dict[old.CORE_ASES_STRING].items():
        core_dict[ia] = {
            new.OFFLINE_KEY_STRING: base64.b64encode(d[old.OFFLINE_KEY_STRING]).decode(),
            new.OFFLINE_KEY_ALG_STRING: d[old.OFFLINE_KEY_ALG_STRING].lower(),
            new.ONLINE_KEY_STRING: base64.b64encode(d[old.ONLINE_KEY_STRING]).decode(),
            new.ONLINE_KEY_ALG_STRING: d[old.ONLINE_KEY_ALG_STRING].lower(),
        }

    quorum_cas = 0  # old_dict[old.QUORUM_CAS_STRING]
    quorum_trc = min(old_dict[old.QUORUM_OWN_TRC_STRING], len(core_dict))

    new_dict = {
        new.ISD_STRING: old_dict[old.ISDID_STRING],
        new.DESCRIPTION_STRING: old_dict[old.DESCRIPTION_STRING],
        new.VERSION_STRING: old_dict[old.VERSION_STRING],
        new.CREATION_TIME_STRING: old_dict[old.CREATION_TIME_STRING],
        new.EXPIRATION_TIME_STRING: old_dict[old.CREATION_TIME_STRING] + d_to_s(val_period),
        new.CORE_ASES_STRING: core_dict,
        new.ROOT_CAS_STRING: {},
        new.CERT_LOGS_STRING: {},
        new.THRESHOLD_EEPKI_STRING: old_dict[old.QUORUM_EEPKI_STRING],
        new.RAINS_STRING: {},
        new.QUORUM_TRC_STRING: quorum_trc,
        new.QUORUM_CAS_STRING: quorum_cas,
        new.QUARANTINE_STRING: old_dict[old.QUARANTINE_STRING],
        new.SIGNATURES_STRING: {},
        new.GRACE_PERIOD_STRING: old_dict[old.GRACE_PERIOD_STRING],
    }
    return TRC(new_dict)


def sign_trc(trc, keys):
    """
    Sign TRC with provided keys.
    They keys dict needs to contain at least the of all core ASes in the ISD.

    :param TRC trc: TRC in new format
    :param dict keys: Mapping ISD-AS -> online key containing all necessary keys
    :return: Signed TRC
    :rtype: TRC
    """
    for ia in trc.core_ases:
        try:
            trc.sign(ia, keys[ia])
        except KeyError:
            fatal("Online key missing for %s" % ia)
    return trc


def write_trc(trcs, mode, outdir, suffix, pre_elem):
    """
    Write TRC files to file system.
    If mode=OVR -> overwrite original TRC file with new one.
    If mode=SUF -> write TRC in the same dir as original TRC with suffix appended to name.
    If mode=OUT -> write TRC to outdir. If pre_elem=True write to subdir outdir/element_name.
    pre_elem requires, that the original TRC is in a conf dir.
    (i.e. path resembles: .../cs1-10-1/certs/ISD1-V0.trc)

    :param list((str, TRC)) trcs: List of (orig path, TRC)-tuples
    :param str mode: Write mode
    :param str outdir: Outdir path
    :param str suffix: Suffix
    :param bool pre_elem: Add subdir for element
    """
    ctr = 0
    if mode == OUT:
        os.makedirs(outdir, exist_ok=True)
    for (file, trc) in trcs:
        if mode == SUF:
            path = "%s.%s" % (file, suffix)
        elif mode == OVR:
            path = file
        else:
            elem = ""
            if pre_elem:
                elem = os.path.split(os.path.split(os.path.split(file)[0])[0])[1]
            path = os.path.join(outdir, elem, "ISD%s-V%s.trc" % trc.get_isd_ver())
        write_file(path, str(trc))
        ctr += 1
    log.info("%d files written" % ctr)


def parse_mode(mode):
    m = mode.upper()
    if m not in [OUT, OVR, SUF]:
        fatal("Provided mode %s is not valid" % mode)
    return m


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-l', '--loglevel', default='INFO',
                        help='Console logging level (Default: %(default)s)')
    parser.add_argument('-p', '--period', default=365, type=int, help='TRC validity period in days')
    parser.add_argument('-o', '--outdir', type=str, default='translated_certs',
                        help='Output directory (Default: %(default)s)')
    parser.add_argument('-e', '--element', action='store_true', help="add element subdir to outdir")
    parser.add_argument('-s', '--suffix', type=str, default='test',
                        help='Suffix used if mode==SUFFIX (Default: %(default)s) '
                             '!!! DO NOT USE new as suffix !!!')
    parser.add_argument('-c', '--confdirs', nargs="+", help='List of conf dirs for all required '
                                                            'core ASes')
    parser.add_argument('--mode', type=str, default='OUTDIR', help="""
            Output mode for new certificates.
            Available Modes: [OUTDIR, OVERWRITE, SUFFIX]
            OUTDIR: New chains are written to specified outdir.
            OVERWRITE: New chains overwrite the old files.
            SUFFIX: New chains are written to same dir as originals with suffix added to file names.
            """)
    parser.add_argument('trcs', nargs="+", help='TRC files in old format')
    args = parser.parse_args()
    if args.loglevel not in LVLS:
        fatal("Invalid loglevel. Available: [%s, %s, %s, %s]" % LVLS)
    log.set_level(args.loglevel)
    mode = parse_mode(args.mode)
    trcs = update_trcs(args.trcs, args.period, load_keys(args.confdirs))
    write_trc(trcs, mode, args.outdir, args.suffix, args.element)


def fatal(msg):
    log.error(msg)
    exit(-1)

if __name__ == '__main__':
    main()
