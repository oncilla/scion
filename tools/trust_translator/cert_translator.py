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
:mod:`cert_translator` --- Translate outdated certificate chains to the correct format
===========================================
"""

# Stdlib
import argparse
import base64
import os


from lib.crypto.asymcrypto import get_sig_key
from lib.crypto.certificate import Certificate, SUBJECT_SIG_KEY_STRING
from lib.crypto.certificate_chain import CertificateChain
from lib.crypto.util import get_online_key_file_path
from lib.defines import TOPO_FILE
from lib.errors import SCIONIOError
from lib.packet.scion_addr import ISD_AS
from lib.topology import Topology
from lib.util import read_file, write_file
from tools.trust_translator.logger import LVLS, Logger
from tools.trust_translator.old_ver.old_certificate import (
    SUBJECT_SIG_KEY_STRING as OLD_SUBJECT_SIG_KEY_STRING
)
from tools.trust_translator.old_ver.old_certificate_chain import (
    CertificateChain as OldChain
)

OUT = "OUTDIR"
OVR = "OVERWRITE"
SUF = "SUFFIX"
log = Logger()


def load_keys(conf_dirs):
    """
    Loads core AS signing key and online key. Checks that provided conf dir is form a core AS.

    :param str conf_dirs: List of configuration directories of issuing core AS.
    :return: Mapping issuing ISD-AS -> (signing key, online key)
    :rtype: dict
    """
    log.info("Loading keys")

    keys = {}
    for conf_dir in conf_dirs:
        topo = Topology.from_file(os.path.join(conf_dir, TOPO_FILE))
        if not topo.is_core_as:
            log.debug("Provided conf is not from a core AS.")
            continue
        try:
            sig_key = get_sig_key(conf_dir)
            on_key = base64.b64decode(read_file(get_online_key_file_path(conf_dir)))
            keys[str(topo.isd_as)] = (sig_key, on_key)
        except SCIONIOError as e:
            fatal("%s.\nMake sure to confdir has already been translated" % str(e))
    return keys


def update_chains(files, keys):
    """
    Load certificate chains in old format from files and translates them to the new format.
    Additionally, the chains are signed by the provided keys.

    :param list(str) files: List of old certificates
    :param dict keys: Mapping issuing ISD-AS -> (signing key, online key)
    :return: list of (original file name, signed certificate chain in new format)
    :rtype: list((str, CertificateChain))
    """
    log.info("Updating chains")
    chains = []
    for file in files:
        log.debug("Handling %s" % file)
        chain = translate_chain(file)
        ia = chain.as_cert.issuer
        if ia not in keys:
            fatal("Certificate issuer for %sv%d not in keys. ISD-AS: %s" % (
                chain.as_cert.subject, chain.as_cert.version, ia))
        chain.as_cert.sign(keys[ia][0])
        chain.core_as_cert.sign(keys[ia][1])
        chains.append((file, chain))
    return chains


def translate_chain(file):
    """
    Load certificate chain in old format from file and translate to new format.

    :param str file: Old certificate chain
    :returns: Certificate chain in the new format
    :rtype: CertificateChain
    """
    try:
        f = open(file, "r")
    except FileNotFoundError:
        fatal("File '%s' does not exist. Abort" % file)
    old_chain = OldChain.from_raw(f.read())
    f.close()

    leaf_cert = translate_cert(old_chain.as_cert)
    core_cert = translate_cert(old_chain.core_as_cert)
    return CertificateChain([leaf_cert, core_cert])


def translate_cert(old_cert):
    """
    Translate certificate from old format to new one.

    :param Certificate old_cert: Certificate in old format
    :return: Certificate in new format
    :rtype: Certificate
    """
    d = old_cert.dict()
    d[SUBJECT_SIG_KEY_STRING] = d[OLD_SUBJECT_SIG_KEY_STRING]
    return Certificate(d)


def write_chains(chains, mode, outdir, suffix, pre_elem):
    """
    Write certificate chain files to file system.
    If mode=OVR -> overwrite original chain file with new one.
    If mode=SUF -> write chain in the same dir as original chain with suffix appended to name.
    If mode=OUT -> write chain to outdir. If pre_elem=True write to subdir outdir/element_name.
    pre_elem requires, that the original chain is in a conf dir.
    (i.e. path resembles: .../cs1-10-1/certs/ISD1-AS10-V0.crt)

    :param list((str, CertificateChain)) chains: List of (orig path, CertificateChain)-tuples
    :param str mode: Write mode
    :param str outdir: Outdir path
    :param str suffix: Suffix
    :param bool pre_elem: Add subdir for element
   """
    ctr = 0
    if mode == OUT:
        os.makedirs(outdir, exist_ok=True)
    for (file, chain) in chains:
        if mode == SUF:
            path = "%s.%s" % (file, suffix)
        elif mode == OVR:
            path = file
        else:
            ia = ISD_AS(chain.as_cert.subject)
            elem = ""
            if pre_elem:
                elem = os.path.split(os.path.split(os.path.split(file)[0])[0])[1]
            path = os.path.join(outdir, elem, "ISD%s-AS%s-V%s.crt" % (
                ia[0], ia[1], chain.as_cert.version))
        write_file(path, str(chain))
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
    parser.add_argument('-c', '--confdirs', nargs='+', required=True,
                        help='Issuing Core AS configuration dirs (Required)')
    parser.add_argument('-o', '--outdir', type=str, default='translated_certs',
                        help='Output directory (Default: %(default)s)')
    parser.add_argument('-e', '--element', action='store_true', help="add element subdir to outdir")
    parser.add_argument('-s', '--suffix', type=str, default='test',
                        help='Suffix used if mode==SUFFIX (Default: %(default)s) '
                             '!!! DO NOT USE new as suffix !!!')
    parser.add_argument('--mode', type=str, default='OUTDIR', help="""
            Output mode for new certificates.
            Available Modes: [OUTDIR, OVERWRITE, SUFFIX]
            OUTDIR: New chains are written to specified outdir.
            OVERWRITE: New chains overwrite the old files.
            SUFFIX: New chains are written to same dir as originals with suffix added to file names.
            """)
    parser.add_argument('chains', nargs='+', help='certificate chain files in old format issued '
                                                  'by core AS')
    args = parser.parse_args()
    if args.loglevel not in LVLS:
        fatal("Invalid loglevel. Available: [%s, %s, %s, %s]" % LVLS)
    log.set_level(args.loglevel)
    mode = parse_mode(args.mode)
    chains = update_chains(args.chains, load_keys(args.confdirs))
    write_chains(chains, mode, args.outdir, args.suffix, args.element)


def fatal(msg):
    log.error(msg)
    exit(-1)

if __name__ == '__main__':
    main()
