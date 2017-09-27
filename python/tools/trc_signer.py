# Copyright 2017 ETH Zurich
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
:mod:`trc_signer` --- Script to sign TRC files
==================================================
"""
# Stdlib
import base64
import getopt
import os
import sys

# External packages
from nacl.signing import SigningKey

# SCION
from lib.crypto.trc import TRC
from lib.crypto.util import get_online_key_file_path
from lib.defines import TOPO_FILE
from lib.topology import Topology
from lib.util import read_file, write_file

HELP_TEXT = '%s -i <inputfile> -o <outputfile> -c <confdir>' % sys.argv[0]


def sign_trc(conf_dir, infile, outfile):
    """
    Sign TRC and write it to outfile.

    :param string conf_dir: path of configuration directory.
    :param string infile: path of TRC to be signed.
    :param String outfile: path where signed TRC is written to.
    """
    topology = Topology.from_file(os.path.join(conf_dir, TOPO_FILE))
    sign_key = base64.b64decode(read_file(get_online_key_file_path(conf_dir)))
    trc = TRC.from_raw(read_file(infile))
    trc.sign(str(topology.isd_as), sign_key)  # FIXME(roosd): remove str() when PR1144 is merged
    write_file(outfile, str(trc))


def main(argv):
    infile = outfile = conf_dir = ''
    try:
        opts, args = getopt.getopt(argv, "hi:o:c:", ["ifile=", "ofile=", "conf="])
    except getopt.GetoptError:
        print(HELP_TEXT)
        sys.exit(2)
    for opt, arg in opts:
        if opt == '-h':
            print(HELP_TEXT)
            sys.exit()
        elif opt in ("-i", "--ifile"):
            infile = arg
        elif opt in ("-o", "--ofile"):
            outfile = arg
        elif opt in ("-c", "--conf"):
            conf_dir = arg

    if not infile or not conf_dir:
        print(HELP_TEXT)
        sys.exit(2)
    if not outfile:
        outfile = infile
    sign_trc(conf_dir, infile, outfile)


if __name__ == "__main__":
    main(sys.argv[1:])
