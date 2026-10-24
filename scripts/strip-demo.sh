#!/bin/sh
# Removes the demo (the items list, its worker and its tests) so you start clean.
#
#   scripts/strip-demo.sh
#
# Demo code is marked DEMO:
#   - files that start with a "DEMO:" comment are deleted
#   - lines that end in a DEMO comment (// DEMO, # DEMO, <!-- DEMO -->) are removed
#   - blocks between "DEMO start" and "DEMO end" lines are removed
# Commit or stash your changes first, so you can review the result with git diff.
# The script deletes itself when it is done.
set -eu

cd "$(dirname "$0")/.."
command -v git >/dev/null || { echo "Error: git is required" >&2; exit 1; }
command -v perl >/dev/null || { echo "Error: perl is required" >&2; exit 1; }

self=scripts/strip-demo.sh
# -I skips binary files; the script itself mentions the markers, so it is skipped too
git grep -l -I -e 'DEMO' -- . ":(exclude)$self" | while IFS= read -r f; do
	if head -n 5 "$f" | grep -q 'DEMO:'; then
		git rm -qf "$f"
		echo "deleted   $f"
	else
		# also drops a blank line left doubled where something was removed
		perl -ni -e '
			if (/DEMO start/) { $skip = $cut = 1; next }
			if (/DEMO end/) { $skip = 0; next }
			next if $skip;
			if (m{(//|#|<!--)\s*DEMO\s*(-->)?\s*$}) { $cut = 1; next }
			next if $cut && /^\s*$/ && $blank;
			$cut = 0;
			$blank = /^\s*$/;
			print;
		' "$f"
		echo "cleaned   $f"
	fi
done

if command -v gofmt >/dev/null; then gofmt -w backend; fi
git rm -q "$self"

cat <<'MSG'

The demo is gone; review it with `git diff`, then run `make check test`.
A database that already has the items collection keeps it: delete it in the
admin UI (/_/) under Collections.
MSG
