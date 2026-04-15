# Proposal: Remove the init container via image volumes

## Status

Draft — proposed as an opt-in feature.

## Summary

Today every Argo workflow pod runs an init container (`argoexec init`) that:

1. Copies the `argoexec` binary onto a shared `emptyDir` so `main` can use it as its entrypoint (the "emissary").
2. Writes the template JSON to the shared volume.
3. Stages script source (`StageFiles`) for `script` templates.
4. Downloads input artifacts (`LoadArtifactsWithoutPlugins`).

This proposal eliminates that init container by:

- Providing the `argoexec` binary to `main` via a Kubernetes **image volume** (KEP-4639), instead of copying it via init.
- Moving template-JSON write, script staging, and input artifact download into the existing `wait` container.
- Having `main`'s emissary block on a ready marker written by `wait` to the shared volume.

The net effect is one fewer container per pod, faster pod startup (no sequential init phase), and a simpler mental model.

## Motivation

- Init containers run sequentially before any regular container, adding latency to every workflow pod.
- The init container's responsibilities overlap conceptually with `wait` — both are Argo-infrastructure containers running `argoexec`. Consolidating them reduces moving parts.
- Image volumes are the idiomatic Kubernetes way to ship a binary into a container without modifying the user's image or running an init copy step.

## Non-goals

- Changing the user-facing workflow spec.
- Removing the `wait` container.
- Changing how output artifacts / logs / outputs are handled (still in `wait`, post-main).
- Supporting Kubernetes versions below the image-volume floor (see compatibility).

## Design

### Kubernetes features required

| Feature | KEP | Status | Notes |
| --- | --- | --- | --- |
| Image volume source | KEP-4639 | Beta in 1.33, `ImageVolume` feature gate **enabled by default** | Required to mount argoexec image into `main` |

No sidecar-container (KEP-753) dependency is needed — see "Why not sidecars" below.

### Container layout (new mode)

```
pod:
  volumes:
    - name: argoexec-bin
      image:
        reference: quay.io/argoproj/argoexec:<tag>
        pullPolicy: IfNotPresent
    - name: var-run-argo
      emptyDir: {}

  # NO init containers (except artifact-plugin inits — see open questions)

  containers:
    - name: wait
      image: quay.io/argoproj/argoexec:<tag>
      command: [argoexec, wait]
      volumeMounts:
        - { name: var-run-argo, mountPath: /var/run/argo }
      # wait now performs: template JSON write, StageFiles,
      # LoadArtifactsWithoutPlugins, then writes /var/run/argo/ready
      # (or /var/run/argo/failed). Afterwards proceeds with its
      # existing post-main responsibilities.

    - name: main
      image: <user image>
      command: [/argo-bin/argoexec, emissary, --, <user command...>]
      volumeMounts:
        - { name: argoexec-bin, mountPath: /argo-bin, readOnly: true }
        - { name: var-run-argo, mountPath: /var/run/argo }
      # Emissary polls /var/run/argo/ready before exec'ing user command.
      # Exits non-zero if /var/run/argo/failed appears.
```

### Startup sequence

1. Pod scheduled. Image volume is mounted before containers start.
2. `wait` and `main` start (no ordering guarantee from K8s).
3. `main`'s emissary (`argoexec emissary`) begins polling `/var/run/argo/ready` with a short sleep loop.
4. `wait` in order:
   - Writes `/var/run/argo/template` (template JSON).
   - Calls `StageFiles` (for script templates).
   - Calls `LoadArtifactsWithoutPlugins` (downloads input artifacts).
   - Writes `/var/run/argo/ready` atomically (write-then-rename).
5. Emissary sees `ready`, exec's the user command.
6. `wait` continues its current behavior: waits on `main`, captures outputs/logs/artifacts.

### Failure handling

| Scenario | Behavior |
| --- | --- |
| Artifact download fails in `wait` | `wait` writes `/var/run/argo/failed` containing the error, then exits non-zero. Emissary in `main` sees `failed`, logs the message, exits non-zero. Pod fails. |
| `wait` container crashes before writing either marker | `main`'s emissary polls with a timeout (configurable, default matches current init-container semantics — e.g. the pod's `activeDeadlineSeconds` or a bounded emissary timeout). On timeout, emissary exits non-zero. |
| Image volume pull fails | Kubernetes surfaces `ImagePullBackOff` on `main` — same UX as any other container image failure. |
| User's image is distroless/scratch | Works — `argoexec` from the image volume is the entrypoint; we do not depend on shell in the user image. |

### Why not sidecar containers (KEP-753)?

An earlier sketch used native sidecars to guarantee `wait` is running before `main`. It's unnecessary: the emissary already runs as `main`'s PID 1 and is designed to block before exec'ing the user command. Polling for a marker file is the same mechanism already used elsewhere in the emissary. Avoiding the sidecar dependency lowers the K8s version floor and keeps `wait` as a normal container with its existing lifecycle.

### Opt-in mechanism

This is a behavioral change that requires image-volume support on every node that runs workflow pods, so it must be **opt-in** initially.

Controller-level toggle in the workflow controller ConfigMap:

```yaml
containerRuntimeExecutor: emissary  # existing
podSpecPatch: ...                    # existing

# NEW
initlessPod:
  enabled: false                     # default off
  argoexecImage: ""                  # optional override; defaults to controller's argoexec image
  imageVolumePullPolicy: IfNotPresent
  readyMarkerTimeout: 5m             # how long main's emissary waits for wait to signal ready
```

Per-workflow override on `WorkflowSpec`:

```yaml
spec:
  initlessPod: true    # or false to force-disable when controller default is on
```

When `initlessPod` is false (default), behavior is unchanged. When true, the controller emits the new pod layout and the image-volume version check must pass (see below).

### Kubernetes version / feature-gate detection

On controller startup, and again lazily on pod-spec generation:

- Query `/version` to confirm server ≥ 1.33.
- Attempt a dry-run pod create with an image-volume source the first time the feature is used per cluster; cache the result.
- If unavailable, fail workflow submission with a clear error (`initlessPod requires Kubernetes 1.33+ with the ImageVolume feature gate enabled`) rather than silently falling back — silent fallback would hide misconfiguration.

## Implementation plan

### Code changes

1. **`workflow/controller/workflowpod.go`**
   - Branch at pod construction on `initlessPod`.
   - New codepath builds a pod spec with:
     - No `argoexec init` init container.
     - Image volume for argoexec binary.
     - `main` entrypoint pointing at `/argo-bin/argoexec emissary …`.
     - `wait` command augmented with a new pre-phase flag, e.g. `argoexec wait --prepare-main`.
   - Keep the existing codepath intact for when the flag is off.

2. **`cmd/argoexec/commands/wait.go`**
   - Add `--prepare-main` (or fold into default wait behavior when detected via env var set by the controller).
   - Before entering the existing wait loop, perform:
     - Template JSON write (currently in `commands/init.go`).
     - `wfExecutor.StageFiles()`.
     - `wfExecutor.LoadArtifactsWithoutPlugins()`.
   - On success: atomic write of `/var/run/argo/ready`.
   - On failure: write `/var/run/argo/failed` with error text, exit non-zero.

3. **`workflow/executor/emissary/emissary.go`**
   - Before exec'ing the user command, poll for `/var/run/argo/ready` / `/var/run/argo/failed` with the configured timeout.
   - On `failed`: read message, log to stderr, exit non-zero with the same exit code semantics `init` would have produced.
   - On timeout: exit with a distinct error code/message.

4. **Config plumbing**
   - Extend `config.Config` with `InitlessPod` struct.
   - Extend `WorkflowSpec` with the per-workflow opt-in (behind feature-gate validation).
   - Controller startup: probe K8s version / feature gate, log result.

5. **Artifact plugin init containers (`workflowpod.go:779-805`)**
   - Option A (simpler, ship first): keep these init containers. The claim "no init containers" becomes "no argoexec init container"; artifact plugins still use inits. Acceptable but reduces the benefit for plugin users.
   - Option B (follow-up): run plugin init logic inside `wait` as additional pre-main steps. Requires plugin driver refactor.
   - Recommendation: ship A, file a follow-up issue for B.

### Tests

- Unit: pod-spec generation with `initlessPod: true` — verify shape, image volume, no argoexec init, emissary entrypoint, wait command flags.
- Unit: emissary polling behavior — ready, failed, timeout cases.
- E2E (gated on K8s version in CI):
  - Happy-path workflow with script template.
  - Happy-path workflow with input artifacts.
  - Artifact download failure surfaces as pod failure with a useful message.
  - Distroless user image (e.g. `gcr.io/distroless/static`) runs correctly.
  - `initlessPod: false` (default) remains byte-identical to current pod specs.

### Documentation

- New page under `docs/` describing the feature, version requirements, how to enable, and trade-offs.
- `CHANGELOG` entry noting opt-in behavior and K8s 1.33 requirement when enabled.
- Update `docs/architecture.md` to show both pod shapes.

## Compatibility & migration

- Default off. No existing workflow changes behavior.
- When enabled, running workflows are unaffected — only newly scheduled pods use the new layout.
- Rollback: flip the config flag off; next pod uses the old layout. No state migration required.
- Mixed-mode clusters are fine: the controller decides per-pod, the K8s API handles each pod independently.

## Open questions

1. **Artifact plugin init containers** — ship as-is (Option A above), or block on refactor (Option B)?
2. **PNS executor interaction** — PNS relies on `wait` seeing `main`'s PID/filesystem. Regular containers share no ordering, but PNS today works because `wait` is also a regular container. The current comment at `workflowpod.go:272` ("For PNS we want the wait container to start before the main") suggests some ordering reliance. Needs verification that PNS still functions when both start concurrently and `main` blocks in emissary.
3. **Timeout tuning** — what's a sensible default for `readyMarkerTimeout`? Must accommodate worst-case artifact download from slow repos. Could default to the existing artifact-download timeout and be overridable per workflow.
4. **Image volume pull semantics** — confirm that `imagePullSecrets` on the pod apply to image-volume sources (they do per KEP-4639, but worth validating in practice).
5. **Emissary binary architecture** — image-volume image must match the node's architecture. Multi-arch manifests handle this, but should be documented for users building custom argoexec images.

## Alternatives considered

- **Native sidecar containers (KEP-753) for `wait`** — rejected as unnecessary given emissary's existing blocking semantics; would raise the K8s version floor without benefit.
- **Shell wrapper in `main` that waits for a binary copy** — rejected because it breaks distroless/scratch user images.
- **Baking argoexec into the user image** — rejected as intrusive and incompatible with arbitrary user images.
- **Status quo** — the init container works, but consolidating into `wait` removes a container per pod and a sequential startup phase, which is worthwhile at fleet scale.
