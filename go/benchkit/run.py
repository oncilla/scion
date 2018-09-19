#!/usr/bin/env python3

import sys
import subprocess


def main():
    cmd = sys.argv[1:]
    for i, f in enumerate(cmd):
        print(i,f)
        if f == "clean":
            subprocess.call("rm bench/*")
        if f == "bench":
            subprocess.call(["go run sibra_benchkit.go timeplot.go"])
        if f == "plot":
            subprocess.call(["plotter.py", ])


if __name__ == "__main__":
    main()
