#!/usr/bin/env python3
"""Buffer binary load-test output before writing to a mounted filesystem."""
import shutil
import sys

with open(sys.argv[1], "wb", buffering=1024 * 1024) as output:
    shutil.copyfileobj(sys.stdin.buffer, output, length=1024 * 1024)
