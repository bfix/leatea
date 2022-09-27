#!/bin/bash

GOMEMLIMIT=4096MiB GOMAXPROCS=$(($(nproc)-2)) ./liti -c tests/config-$1.json -p rt/leatea.pprof 2>&1 | tee rt/log
