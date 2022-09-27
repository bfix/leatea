#!/bin/bash

GOMAXPROCS=$(($(nproc)-2)) ./liti -c tests/config-$1.json 2>&1 | tee rt/log
