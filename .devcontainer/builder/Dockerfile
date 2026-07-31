# Everything a contributor needs is in the development shell (see flake.nix), so
# this image only has to provide Nix and a store that already holds that shell.
# Building it here rather than on first use is what keeps provisioning to about
# a minute, and it means the container cannot drift from the flake: there is no
# second list of tools to keep in step.

# TODO: Upgrade to Ubuntu 24.04 when https://bugs.launchpad.net/ubuntu/+source/curl/+bug/2073448 is addressed
FROM mcr.microsoft.com/vscode/devcontainers/base:ubuntu-22.04

# Same release the CI jobs get from cachix/install-nix-action, see
# .github/actions/setup-nix.
# renovate: datasource=github-tags depName=NixOS/nix
ARG NIX_VERSION=2.35.1

# The Nix binary tarball is xz-compressed and the base image has no xz.
RUN apt-get update \
  && apt-get install --yes --no-install-recommends xz-utils \
  && rm -rf /var/lib/apt/lists/*

# Nix is installed single-user and owned by `vscode`: a container has no init
# system to run the multi-user daemon under, and `vscode` is the only user that
# ever builds anything here. Creating the store up front is what tells the
# installer that, instead of it asking sudo for a daemon install.
RUN install --directory --mode 0755 --owner vscode --group vscode /nix

# `sandbox` — a single-user Nix has no daemon, so it would have to set up the
#   build sandbox itself, which an unprivileged process in a container cannot do.
# `extra-substituters` — the cache .github/workflows/nix.yaml fills. Without it
#   the warm-up below builds every pinned tool from source.
RUN mkdir -p /etc/nix && printf '%s\n' \
  'experimental-features = nix-command flakes' \
  'sandbox = false' \
  'extra-substituters = https://argo-workflows.cachix.org' \
  'extra-trusted-public-keys = argo-workflows.cachix.org-1:6SFfGYWkzLxEQhKynPaXUtXFTHipK2Ib2kmKFz+K0BA=' \
  > /etc/nix/nix.conf

# So that Nix is on PATH for every shell in the container, whether or not it
# reads the profile scripts the installer writes.
ENV PATH=/home/vscode/.nix-profile/bin:${PATH}

USER vscode

# The versioned installer URL is immutable and the script it serves verifies the
# SHA-256 of the tarball it downloads, so the version above is the whole pin.
RUN curl --fail --silent --show-error --location \
  "https://releases.nixos.org/nix/nix-${NIX_VERSION}/install" \
  | sh -s -- --no-daemon --no-channel-add

# Only the three files the flake reads. Copying the whole repo would rebuild the
# layer below on every commit, and the workspace is bind-mounted at run time
# anyway.
COPY --chown=vscode:vscode flake.nix flake.lock go.mod /tmp/flake/

# Realise the development shell into the store. Evaluating the same three files
# from the workspace produces the same derivation, so `nix develop` there finds
# every path already built. The profile is a GC root, so the shell survives a
# `nix-collect-garbage` inside the container.
RUN mkdir -p /home/vscode/.local/state/nix/profiles \
  && nix develop --profile /home/vscode/.local/state/nix/profiles/argo-devshell \
    /tmp/flake --command true \
  && rm -rf /tmp/flake

# Interactive shells start in the development shell. Ubuntu's .bashrc has
# already returned for non-interactive ones by this point.
#
# The shell `nix develop` starts sources this file again, and it does so before
# it exports IN_NIX_SHELL, so that is no use as the guard against recursing —
# hence a variable of our own, exported before the call. Not `exec`, so that
# someone who has just broken flake.nix gets a prompt to fix it from rather than
# a terminal that closes itself.
RUN printf '%s\n' \
  '' \
  '# Tools come from the Nix development shell (see flake.nix), not from the image.' \
  'if [ -z "$ARGO_DEV_SHELL" ] && [ -f "$PWD/flake.nix" ]; then' \
  '  export ARGO_DEV_SHELL=1' \
  '  nix develop || echo "the development shell did not start: no project tools on PATH" >&2' \
  'fi' \
  >> /home/vscode/.bashrc

# Back to the base image's user: the dev container CLI adds its features on top
# of this image and expects to start from the user the base image left behind.
USER root
