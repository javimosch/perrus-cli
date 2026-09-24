#!/bin/bash
# failed-units-watch — alert on systemd units that newly entered the failed
# state, through perrus-cli's /api/v1/ingest (so it reuses perrus's Telegram
# notifier instead of carrying its own bot config).
#
# perrus only probes URLs, so a timer job that fails on every run is invisible
# to it: a report job on one host failed for two months while the page it
# published looked current. This closes that gap for the host it runs on.
#
# Read-only apart from its state file: it never restarts or resets a unit.
# - First run: records what is already failed and alerts on nothing.
# - Later runs: alerts once per unit when it enters the failed state; a unit
#   that recovers and fails again alerts again.
# - If the alert is not delivered (perrus down, cooldown, Telegram error) the
#   state is left alone, so the next run retries.
#
# Blind spot: a oneshot that fails and succeeds again between two runs is
# never seen as failed. Run it at least as often as the jobs you care about.
set -euo pipefail

INGEST_URL=${PERRUS_INGEST_URL:-http://127.0.0.1:9191/api/v1/ingest}
STATE=${FAILED_UNITS_STATE:-/var/lib/failed-units-watch/failed}
SOURCE=${FAILED_UNITS_SOURCE:-$(hostname -s)/failed-units}
IGNORE_RE=${FAILED_UNITS_IGNORE:-'^run-u[0-9]+\.service$'}  # transient systemd-run units

mkdir -p "$(dirname "$STATE")"
now=$(systemctl --failed --no-legend --plain | awk '{print $1}' | grep -Ev "$IGNORE_RE" | sort -u || true)

if [ ! -f "$STATE" ]; then
  printf '%s\n' "$now" > "$STATE"
  echo "seeded: ${now:-nothing failed}" | tr '\n' ' '; echo
  exit 0
fi

new=$(comm -13 <(sort -u "$STATE") <(printf '%s\n' "$now") | grep -v '^$' || true)
if [ -z "$new" ]; then
  printf '%s\n' "$now" > "$STATE"
  exit 0
fi

text="Failed units on $(hostname -s):"
for u in $new; do
  text+=$'\n\n'"• $u — $(systemctl show "$u" -p Description --value)"
  text+=$'\n'"$(journalctl -u "$u" -n 3 --no-pager -o cat 2>/dev/null | cut -c1-200)"
done

payload=$(python3 -c 'import json,sys; print(json.dumps({"source":sys.argv[1],"level":"ALERTE","text":sys.argv[2]}))' "$SOURCE" "$text")
resp=$(curl -sf -m 20 -X POST -H 'Content-Type: application/json' --data "$payload" "$INGEST_URL") || { echo "ingest unreachable; will retry" >&2; exit 1; }
case "$resp" in
  *'"cooldown"'*) echo "ingest cooldown; will retry" >&2; exit 0 ;;
esac
printf '%s\n' "$now" > "$STATE"
echo "alerted: $new" | tr '\n' ' '; echo
