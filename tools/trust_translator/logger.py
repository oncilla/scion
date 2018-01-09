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

DBG = "DEBUG"
INF = "INFO"
WAR = "WARN"
ERR = "ERROR"
LVLS = (DBG, INF, WAR, ERR)


class Logger:

    def __init__(self, level=None):
        self.lvl = level
        if not level:
            self.lvl = DBG

    def set_level(self, level):
        if level in LVLS:
            self.lvl = level

    def debug(self, msg):
        if self.lvl in (DBG):
            print("DEBUG:", msg)

    def info(self, msg):
        if self.lvl in (DBG, INF):
            print("INFO:", msg)

    def warn(self, msg):
        if self.lvl in (DBG, INF, WAR, ERR):
            print("WARN:", msg)

    def error(self, msg):
        if self.lvl in (DBG, INF, ERR):
            print("ERROR:", msg)
