#!/bin/bash

cfg=${1:-rand}
shift

# additional arguments:
#   -p rt/leatea.pprof    generate profile

GOMEMLIMIT=4096MiB GOMAXPROCS=$(($(nproc)-2)) ./liti -c tests/config-${cfg}.json $* 2>&1 > rt/log &
psrecord $(pgrep liti) --interval 1 --plot rt/monitor.png
