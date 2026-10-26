#!/usr/bin/env bash
set -euo pipefail

if [[ ${1:-} =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    version=$1
else
    echo "usage: $0 <version, for example 4.0.1>" >&2
    exit 2
fi

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
vendor_dir="$repo_dir/web/assets/vendor/vditor"
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

npm pack "vditor@$version" --pack-destination "$work_dir" --loglevel=error >/dev/null
archive="$work_dir/vditor-$version.tgz"
expected_integrity=$(npm view "vditor@$version" dist.integrity)
actual_integrity=$(node -e 'const c=require("crypto"),f=require("fs"); process.stdout.write("sha512-"+c.createHash("sha512").update(f.readFileSync(process.argv[1])).digest("base64"))' "$archive")

if [[ -z $expected_integrity || $actual_integrity != "$expected_integrity" ]]; then
    echo "integrity check failed for vditor@$version" >&2
    exit 1
fi

tar -xzf "$archive" -C "$work_dir"
rsync -a --delete "$work_dir/package/dist/" "$vendor_dir/dist/"
cp "$work_dir/package/LICENSE" "$vendor_dir/LICENSE"
printf '%s\n' "$version" > "$vendor_dir/VERSION"

echo "updated Vditor to $version"
