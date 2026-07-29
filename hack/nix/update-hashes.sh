#!/usr/bin/env bash
#
# Repair the fixed-output hashes in flake.nix.
#
# Every pinned tool records the hash of something it downloads — a source
# tarball, a vendored Go module tree, a Cargo registry snapshot — and those
# change whenever the version does. Bump a version in `toolVersions` and run
# this: it refetches each download and writes the new hashes back.
#
# CI runs it on Renovate's branches (.github/workflows/nix-hashes.yml), so a
# version bump arrives with its hashes already correct.
#
# It refetches whether or not anything changed, because that is the only way to
# be sure: a fixed-output path is named after the hash it is *expected* to have,
# so a stale download of the old version already sits at the path the new one
# would use, and an ordinary build would happily reuse it. A full run therefore
# takes a few minutes; name tools as arguments to do only those.

set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
flake=$repo_root/flake.nix
cd -- "$repo_root"

system=$(nix eval --impure --raw --expr 'builtins.currentSystem')

# Every hash Nix checks for us, as `<package>\t<expected hash>`. A tool that
# does not evaluate at all — the Go overlay throws when nixpkgs lags go.mod —
# has no hashes to fix, so it is skipped rather than taking the run down.
# shellcheck disable=SC2016 # ${...} below is Nix interpolation, not shell
list_hashes='
  packages:
  let
    wanted = [
      (p: p.src.outputHash or null)
      (p: p.goModules.outputHash or null)
      (p: p.cargoHash or null)
    ];
    hashesOf = name:
      let
        found = builtins.tryEval (builtins.concatMap
          (where:
            let hash = builtins.tryEval (where packages.${name}); in
            if hash.success && builtins.isString hash.value && hash.value != ""
            then [ "${name}\t${hash.value}" ]
            else [ ]
          )
          wanted);
      in
      if found.success then found.value else [ ];
  in
  builtins.concatStringsSep "\n"
    (builtins.concatMap hashesOf (builtins.attrNames packages))
'

if [[ $# -gt 0 ]]; then
  tools=("$@")
else
  # Only the tools whose hashes are written down in flake.nix. The rest come
  # from nixpkgs, where a hash is not ours to correct.
  hashes=$(nix eval --raw ".#packages.$system" --apply "$list_hashes")
  tools=()
  while IFS=$'\t' read -r name hash; do
    [[ -n $name ]] || continue
    grep -q -F -- "$hash" "$flake" || continue
    [[ " ${tools[*]} " == *" $name "* ]] || tools+=("$name")
  done <<<"$hashes"

  if [[ ${#tools[@]} -eq 0 ]]; then
    echo "no pinned hashes found in $flake — has it been restructured?" >&2
    exit 1
  fi
fi

echo "refetching: ${tools[*]}"

original=$(mktemp)
trap 'rm -f "$original"' EXIT
cp -- "$flake" "$original"

for tool in "${tools[@]}"; do
  echo "--- $tool"
  # nix-update comes from the flake's own nixpkgs, so this needs no second
  # nixpkgs. `--version=skip` leaves `toolVersions` alone: Renovate owns the
  # versions, this owns the hashes underneath them.
  nix run --inputs-from . nixpkgs#nix-update -- --flake --version=skip "$tool"
done

if diff -u --label a/flake.nix --label b/flake.nix "$original" "$flake"; then
  echo "flake.nix hashes were already up to date"
else
  echo "flake.nix hashes updated, as above"
fi
