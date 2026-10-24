#!/bin/sh
# Turns this copy of the plasto template into your app.
#
#   scripts/rename.sh [name] [owner/repo]
#
# Both default to this repository's GitHub remote: a repo created from the
# template as you/doorbell becomes the app "doorbell". It replaces plasto in the
# Go module path, image names, env vars (PLASTO_* becomes DOORBELL_*), the web
# app's title and manifest, and the docs, starts a fresh README and CLAUDE.md
# intro, and deletes the template's own decision log. The worker kit keeps its
# name (plasto_worker). The script deletes itself when it is done.
set -eu

cd "$(dirname "$0")/.."
command -v git >/dev/null || { echo "Error: git is required" >&2; exit 1; }
command -v perl >/dev/null || { echo "Error: perl is required" >&2; exit 1; }

origin=$(git remote get-url origin 2>/dev/null | sed -n 's#.*github\.com[:/]\([^/]*/[^/]*\)$#\1#p' | sed 's/\.git$//')
repo=${2:-$origin}
name=${1:-$(echo "${repo#*/}" | tr "[:upper:]_." "[:lower:]--")}

[ -n "$repo" ] || { echo "Usage: scripts/rename.sh <name> <owner/repo>" >&2; exit 1; }
echo "$name" | grep -Eq '^[a-z][a-z0-9-]*$' ||
	{ echo "Error: the name must be lowercase letters, digits and dashes: $name" >&2; exit 1; }
echo "$repo" | grep -Eq '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$' ||
	{ echo "Error: the repository must look like owner/repo: $repo" >&2; exit 1; }
[ "$name" != plasto ] || { echo "Error: pick a name other than plasto" >&2; exit 1; }

prefix=$(echo "$name" | tr '[:lower:]-' '[:upper:]_')
image=$(echo "$repo" | tr '[:upper:]' '[:lower:]')
self=scripts/rename.sh
echo "Renaming plasto to $name ($repo, env vars ${prefix}_*)"

git rm -q docs/decisions.md
git rm -rq docs/images  # the template's README screenshots

# -I skips binary files
git grep -l -I -i -e plasto -- . ":(exclude)$self" ":(exclude)scripts/strip-demo.sh" |
	while IFS= read -r f; do
		NAME=$name REPO=$repo IMAGE=$image PREFIX=$prefix perl -pi -e '
			s/plasto_worker/\x01KIT\x01/g;            # the worker kit keeps its name
			s/built on plasto/\x01BUILT\x01/g;
			s{ghcr\.io/kalfasyan/plasto}{ghcr.io/$ENV{IMAGE}}g;
			s{kalfasyan/plasto}{$ENV{REPO}}g;
			s/PLASTO_/$ENV{PREFIX}_/g;
			s/plasto/$ENV{NAME}/g;
			s/Plasto/\u$ENV{NAME}/g;
			s/\x01KIT\x01/plasto_worker/g;
			s/\x01BUILT\x01/built on plasto/g;
		' "$f"
	done

# Parts that are only about the template (marked TEMPLATE start/end), and a
# blank line after each
git grep -l -e 'TEMPLATE start' -- . ":(exclude)$self" | while IFS= read -r f; do
	perl -ni -e '
		if (/TEMPLATE start/) { $skip = 1; next }
		if (/TEMPLATE end/) { $skip = 0; $after = 1; next }
		next if $skip || ($after && /^\s*$/);
		$after = 0;
		print;
	' "$f"
done

# A fresh introduction for README.md and CLAUDE.md
NAME=$name perl -0pi -e '
	s{\A.*?(?=^## )}{# $ENV{NAME}\n\nA self-hosted app built on [plasto](https://github.com/kalfasyan/plasto).\n\n}ms
' README.md
NAME=$name perl -0pi -e '
	s{^\Q$ENV{NAME}\E is a template for self-hosted apps:(.*?)\. Decisions and the build plan are in\n`docs/decisions.md`.\n}{$ENV{NAME} is a self-hosted app built on the plasto template:$1.\n}ms
' CLAUDE.md

git rm -q "$self"

cat <<MSG

Done. Next:
  1. Review with \`git diff\`, then run \`make check test\` and commit.
  2. Pick a brand color and icons (CLAUDE.md says where).
  3. Run scripts/strip-demo.sh to remove the demo when you're ready.
  4. After the first push to main, make the images public so servers can pull
     them: https://github.com/$repo/packages (Package settings > Change visibility).
MSG
