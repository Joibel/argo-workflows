#syntax=docker/dockerfile:1.25

# The Argo binaries are NOT compiled here. They are built on the host — by the
# Nix development shell locally and in CI (`nix develop --command make
# dist/...`) — and this file only assembles them into images. That keeps one
# Go toolchain, the one flake.nix pins, instead of a second one baked into a
# builder image that has to be kept in step with go.mod by hand.
#
# `dist/` is in .dockerignore apart from the binaries these stages COPY, so
# build the binary you need before the image that carries it; `make
# argoexec-image` and friends do that for you.

####################################################################################################

# mailcap ships /etc/mime.types, which argoexec reads to type artifacts. The
# distroless base has no package manager, so take the file from Alpine.
FROM alpine:3.24 AS mime-types
RUN apk add --no-cache mailcap

####################################################################################################

# Delve debugger, copied into the `-dev` images so the controller/server/executor
# can be run under `dlv exec` when Tilt is invoked with `--debug=...`. Pinned to a
# release that supports the Go toolchain it builds against. Dev-only: never used
# by the distroless production targets, and the only compiler in this file.
FROM golang:1.26.1-alpine3.23 AS dlv-build
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/go-delve/delve/cmd/dlv@v1.27.0

####################################################################################################

FROM gcr.io/distroless/static-debian13:latest@sha256:9197324ba51d9cd071af8505989365c006adf9d6d2067eada25aef00abbb5278 AS argoexec-base

COPY --from=mime-types /etc/mime.types /etc/mime.types
COPY hack/ssh_known_hosts /etc/ssh/
COPY hack/nsswitch.conf /etc/

####################################################################################################

FROM argoexec-base AS argoexec-nonroot

USER 8737

COPY --chown=8737 dist/argoexec /bin/

ENTRYPOINT [ "argoexec" ]

####################################################################################################
FROM argoexec-base AS argoexec

COPY dist/argoexec /bin/

ENTRYPOINT [ "argoexec" ]

####################################################################################################

FROM gcr.io/distroless/static-debian13:latest@sha256:9197324ba51d9cd071af8505989365c006adf9d6d2067eada25aef00abbb5278 AS workflow-controller

USER 8737

COPY hack/ssh_known_hosts /etc/ssh/
COPY hack/nsswitch.conf /etc/
COPY --chown=8737 dist/workflow-controller /bin/

ENTRYPOINT [ "workflow-controller" ]

####################################################################################################

FROM gcr.io/distroless/static-debian13:latest@sha256:9197324ba51d9cd071af8505989365c006adf9d6d2067eada25aef00abbb5278 AS argocli

USER 8737

WORKDIR /home/argo

COPY hack/ssh_known_hosts /etc/ssh/
COPY hack/nsswitch.conf /etc/
COPY dist/argo /bin/

ENTRYPOINT [ "argo" ]

####################################################################################################
# Dev-only stages for Tilt. Small alpine base; NOT shipped to users. On change
# Tilt rebuilds these (trivial COPY) and recreates the pod.

FROM alpine:3.24 AS workflow-controller-dev
RUN apk add --no-cache ca-certificates
COPY hack/ssh_known_hosts /etc/ssh/
COPY hack/nsswitch.conf /etc/
COPY dist/workflow-controller /bin/workflow-controller
# Delve, for `tilt up -- --debug=controller` (the Tiltfile wraps the entrypoint).
COPY --from=dlv-build /go/bin/dlv /bin/dlv
# Match the prod image's non-root user so runAsNonRoot is satisfied.
USER 8737
ENTRYPOINT [ "workflow-controller" ]

####################################################################################################

FROM alpine:3.24 AS argocli-dev
RUN apk add --no-cache ca-certificates
WORKDIR /home/argo
COPY hack/ssh_known_hosts /etc/ssh/
COPY hack/nsswitch.conf /etc/
COPY dist/argo /bin/argo
# Delve, for `tilt up -- --debug=server` (the Tiltfile wraps the entrypoint).
COPY --from=dlv-build /go/bin/dlv /bin/dlv
USER 8737
ENTRYPOINT [ "argo" ]

####################################################################################################

FROM alpine:3.24 AS argoexec-dev
RUN apk add --no-cache ca-certificates mailcap
COPY hack/ssh_known_hosts /etc/ssh/
COPY hack/nsswitch.conf /etc/
COPY dist/argoexec /bin/argoexec
USER 8737
ENTRYPOINT [ "argoexec" ]
