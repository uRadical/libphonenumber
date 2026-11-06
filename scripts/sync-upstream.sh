#!/bin/sh
# sync-upstream.sh copies resources/ from a libphonenumber release and
# regenerates everything derived from it.
#
# Usage: scripts/sync-upstream.sh [tag]
#
# With no tag it uses upstream's latest release. It does nothing when that
# release is no newer than the commit resources/UPSTREAM records, so it
# never moves the data backwards; set FORCE=1 to sync anyway. It prints the
# tag it synced, or nothing, so a workflow can tell whether there is a
# change to propose.
set -eu

repo=https://github.com/google/libphonenumber
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

tag=${1:-}
if [ -z "$tag" ]; then
	tag=$(git ls-remote --tags --refs "$repo" 'v*' |
		sed 's|.*refs/tags/||' | sort -V | tail -n 1)
fi
[ -n "$tag" ] || { echo "no upstream release found" >&2; exit 1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Only resources/ is needed, and the full repository is large.
git clone -q --depth 1 --branch "$tag" --filter=blob:none --sparse "$repo" "$tmp/up" 2>/dev/null
git -C "$tmp/up" sparse-checkout set resources
sha=$(git -C "$tmp/up" rev-parse HEAD)
date=$(git -C "$tmp/up" log -1 --format=%cs)

# resources/UPSTREAM's second line is "commit <sha> (<date>[, <tag>])".
current=$(sed -n 's/^commit \([0-9a-f]*\) (\([0-9-]*\).*/\1 \2/p' resources/UPSTREAM)
cur_sha=${current% *}
cur_date=${current#* }
if [ "${FORCE:-}" != 1 ]; then
	if [ "$sha" = "$cur_sha" ]; then
		echo "already at $tag ($sha)" >&2
		exit 0
	fi
	# ISO dates compare correctly as strings.
	if ! expr "$date" \> "$cur_date" >/dev/null; then
		echo "$tag ($date) is not newer than $cur_sha ($cur_date)" >&2
		exit 0
	fi
fi

up=$tmp/up/resources
for f in PhoneNumberMetadata.xml PhoneNumberMetadataForTesting.xml \
	ShortNumberMetadata.xml PhoneNumberAlternateFormats.xml; do
	cp "$up/$f" "resources/$f"
done
for d in geocoding carrier timezones test; do
	rsync -a --delete "$up/$d/" "resources/$d/"
done
printf '%s\ncommit %s (%s, %s)\n' "$repo" "$sha" "$date" "$tag" > resources/UPSTREAM

go generate ./...
echo "$tag"
