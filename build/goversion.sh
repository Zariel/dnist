#!/bin/bash

set -euo pipefail

grep "go 1\." go.mod | awk '{ print $2 }'
