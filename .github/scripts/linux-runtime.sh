#!/usr/bin/env bash

set -euo pipefail

: "${RUNNER_TEMP:?RUNNER_TEMP must name the artifact directory}"

program="$RUNNER_TEMP/ayra-catalogue"
stdout_log="$RUNNER_TEMP/catalogue.stdout.log"
stderr_log="$RUNNER_TEMP/catalogue.stderr.log"

go -C catalogue build -o "$program" .
"$program" >"$stdout_log" 2>"$stderr_log" &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT

found=
for _ in $(seq 1 120); do
	if ! kill -0 "$pid" 2>/dev/null; then
		cat "$stderr_log"
		exit 1
	fi
	if xwininfo -root -tree | grep -Fq '"Controls"'; then
		found=yes
		break
	fi
	sleep 0.25
done

if [[ -z "$found" ]]; then
	echo "No Controls window appeared within 30 seconds."
	cat "$stderr_log"
	exit 1
fi

sleep 15
kill -0 "$pid"
xwininfo -root -tree | grep -F '"Controls"'
