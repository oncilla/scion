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
:mod:`key_translator` --- Translate outdated keys to the correct format
===========================================
"""

# Stdlib
import argparse
import base64
import os

import nacl
from nacl.signing import SigningKey

from lib.crypto.asymcrypto import get_sig_key_raw_file_path, get_sig_key_file_path
from lib.crypto.util import (
    get_offline_key_file_path,
    get_offline_key_raw_file_path,
    get_online_key_file_path,
    get_online_key_raw_file_path,
)
from lib.errors import SCIONIOError
from lib.util import read_file, write_file
from tools.trust_translator.logger import LVLS, Logger


OUT = "OUTDIR"
OVR = "OVERWRITE"
SUF = "SUFFIX"
ONL_SEED = "online-root.seed"
OFF_SEED = "offline-root.seed"
SIG_SEED = "as-sig.seed"
ONL_KEY = "online-root.key"
OFF_KEY = "offline-root.key"
SIG_KEY = "as-sig.key"
raw_map = {ONL_KEY: get_online_key_raw_file_path, OFF_KEY: get_offline_key_raw_file_path,
           SIG_KEY: get_sig_key_raw_file_path}
seed_map = {ONL_KEY: get_online_key_file_path, OFF_KEY: get_offline_key_file_path,
            SIG_KEY: get_sig_key_file_path}
log = Logger()


def update_configs(conf_dirs):
    """
    Load all keys from conf dirs.

    :param list(str) conf_dirs: List of confdirs
    :return: List of (conf dir, {keyString -> bytes})
    :rtype: list((str, dict))
    """
    configs = []
    for conf_dir in conf_dirs:
        keys = load_keys(conf_dir)
        configs.append((conf_dir, keys))
    return configs


def load_keys(conf_dir):
    """
    Load all keys from conf dir.

    :param str conf_dir: Path of conf dir
    :return: Mapping (keyString -> bytes)
    :rtype: dict
    """
    keys = {}
    if not os.path.isdir(conf_dir):
        fatal("Confdir %s does not exist" % conf_dir)
    for k, func in raw_map.items():
        key = load_key(func(conf_dir))
        if key:
            keys[k] = key
    return keys


def load_key(file):
    """
    Load base64 encoded key from file.

    :param str file: Base64 encoded key file
    :return: Decoded key
    :rtype: bytes
    """
    try:
        key = base64.b64decode(read_file(file))
        if len(key) == nacl.bindings.crypto_sign_SEEDBYTES:
            return SigningKey(key)
        return None
    except SCIONIOError:
        return None


def write_configs(configs, mode, outdir, suffix):
    """
    Write key files to file system.
    If mode=OVR -> overwrite original key file with new one.
    If mode=SUF -> write key in the same dir as original key with suffix appended to name.
    If mode=OUT -> write key to outdir. If pre_elem=True write to subdir outdir/element_name.

    :param list((str, dict)) configs: List of (orig path, {keyString -> bytes})-tuples
    :param str mode: Write mode
    :param str outdir: Outdir path
    :param str suffix: Suffix
    :param bool pre_elem: Add subdir for element
    """
    encode = base64.b64encode
    if mode == OUT:
        os.makedirs(outdir, exist_ok=True)
    for (conf_dir, keys) in configs:
        log.debug("Mode is %s" % mode)
        to_write = {}
        if mode == SUF:
            for k, key in keys.items():
                to_write["%s.%s" % (raw_map[k](conf_dir), suffix)] = encode(key._signing_key)
                to_write["%s.%s" % (seed_map[k](conf_dir), suffix)] = encode(key.encode())
        elif mode == OVR:
            for k, key in keys.items():
                to_write[raw_map[k](conf_dir)] = encode(key._signing_key)
                to_write[seed_map[k](conf_dir)] = encode(key.encode())
        else:
            elem = os.path.basename(os.path.normpath(conf_dir))
            base = os.path.join(outdir, elem)
            for k, key in keys.items():
                to_write[raw_map[k](base)] = encode(key._signing_key)
                to_write[seed_map[k](base)] = encode(key.encode())
        for path, key in to_write.items():
            log.debug("Write to file %s" % path)
            write_file(path, key.decode())


def parse_mode(mode):
    m = mode.upper()
    if m not in [OUT, OVR, SUF]:
        fatal("Provided mode %s is not valid" % mode)
    return m


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-l', '--loglevel', default='INFO',
                        help='Console logging level (Default: %(default)s)')
    parser.add_argument('-o', '--outdir', type=str, default='translated_keys',
                        help='Output directory (Default: %(default)s)')
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

    parser.add_argument('confdirs', nargs='+', help="AS configuration dirs")
    args = parser.parse_args()
    if args.loglevel not in LVLS:
        fatal("Invalid loglevel. Available: [%s, %s, %s, %s]" % LVLS)
    log.set_level(args.loglevel)
    mode = parse_mode(args.mode)
    configs = update_configs(args.confdirs)
    write_configs(configs, mode, args.outdir, args.suffix)


def fatal(msg):
    log.error(msg)
    exit(-1)

if __name__ == '__main__':
    main()
