# Every tool needed to develop Argo Workflows, in one devShell — bar the
# devcontainer CLI, which is too heavy to carry for the one CI job that uses
# it and has a shell to itself (see devShells.devcontainer below).
#
#   nix develop          # enter the shell
#   direnv allow         # or let direnv do it for you (see .envrc)
#
# Nix manages *tools* only: nothing here builds Argo itself, that stays
# `make build`. The Makefile is a task runner and finds every tool on `PATH`,
# so there are no `go install` targets left to keep in sync.
#
# Tools come in two tiers:
#
#   pinned    Anything whose version is part of a contract — it decides what
#             generated code, lint results or test output look like. These are
#             written out below with an explicit version and hash, and bumping
#             one is a reviewable change.
#
#   floating  Everything else: editors, clients, the local cluster. These come
#             from nixpkgs stable as-is and move when the pin below moves.
{
  description = "Argo Workflows development tooling";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs = { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;

      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];

      # ---------------------------------------------------------------------
      # Pinned tool versions.
      #
      # Renovate keeps these current; each is used exactly once below, so the
      # version and its hash sit next to each other.
      # ---------------------------------------------------------------------
      toolVersions = {
        buf = "1.65.0";
        codeGenerator = "0.35.1";
        controllerTools = "0.18.0";
        cspell = "9.7.0";
        gogoProtobuf = "1.3.2";
        goimports = "0.35.0";
        golangciLint = "2.11.3";
        goSwagger = "0.33.1";
        gotestsum = "1.12.3";
        grpcGateway = "1.16.0";
        kubeauto = "0.0.7";
        kubeOpenapi = "0.0.0-20220124234850-424119656bbf";
        mockery = "3.5.1";
        openapiGenerator = "5.2.1";
        properdocs = "1.6.7";
        snipdoc = "0.1.12";
      };

      # ---------------------------------------------------------------------
      # Go toolchain
      #
      # go.mod is the single source of truth for the Go version. nixpkgs is
      # used when it is new enough; when it lags, we build that exact Go from
      # source instead of silently developing against a different compiler.
      # ---------------------------------------------------------------------
      goVersion =
        let
          line = lib.findFirst (l: lib.hasPrefix "go " l) null
            (lib.splitString "\n" (builtins.readFile ./go.mod));
        in
        if line == null
        then throw "no `go` directive found in go.mod"
        else lib.removePrefix "go " line;

      # Hashes for https://go.dev/dl/go<version>.src.tar.gz. Only needed while
      # nixpkgs lags go.mod — the error below tells you when, and Nix prints the
      # hash to paste in on the first build.
      goSrcHashes = { };

      goOverlay = final: prev:
        let
          majorMinor = lib.versions.majorMinor goVersion;
          attr = "go_${lib.versions.major goVersion}_${lib.versions.minor goVersion}";
          candidate = prev.${attr} or null;
          # A newer patch release of the same minor is fine (and is what the
          # `go` directive means); a different minor is not.
          fromNixpkgs = candidate != null
            && lib.versions.majorMinor candidate.version == majorMinor
            && !lib.versionOlder candidate.version goVersion;

          hash = goSrcHashes.${goVersion} or null;
          fromSource =
            if hash == null then
              throw ''
                go.mod requires Go ${goVersion}, which nixpkgs does not provide
                (nixpkgs.${attr} = ${if candidate == null then "missing" else candidate.version}).

                Add the hash of https://go.dev/dl/go${goVersion}.src.tar.gz to
                `goSrcHashes` in flake.nix and it will be built from source. Use
                lib.fakeHash first; Nix prints the real one.
              ''
            else
              prev.go.overrideAttrs (old: {
                version = goVersion;
                src = prev.fetchurl {
                  url = "https://go.dev/dl/go${goVersion}.src.tar.gz";
                  inherit hash;
                };
              });

          go = if fromNixpkgs then candidate else fromSource;
        in
        {
          inherit go;
          # Keep the tools we build below on the same toolchain as the shell.
          # When nixpkgs is current this is the derivation it already used, so
          # it costs nothing and everything still comes from the binary cache.
          buildGoModule = prev.buildGoModule.override { inherit go; };
        };

      # ---------------------------------------------------------------------
      # Pinned tools
      # ---------------------------------------------------------------------
      toolsOverlay = final: prev: {
        argoTools =
          let
            inherit (final) buildGoModule fetchFromGitHub;
            pythonPkgs = final.python3Packages;

            # ProperDocs is the MkDocs fork the docs are built with (see
            # docs/requirements.txt — keep the version aligned). It is not in
            # nixpkgs, so build it from its PyPI wheel; the Material theme,
            # mkdocs-redirects and pymdown-extensions come from nixpkgs, which
            # also pulls in the `mkdocs` those plugins import at runtime.
            properdocs = pythonPkgs.buildPythonPackage rec {
              pname = "properdocs";
              version = toolVersions.properdocs;
              format = "wheel";
              src = pythonPkgs.fetchPypi {
                inherit pname version format;
                dist = "py3";
                python = "py3";
                hash = "sha256-b6DPouAb8zj2hIksilBs9w6oiufzR5yTO2+iAWgQHL0=";
              };
              propagatedBuildInputs = with pythonPkgs; [
                click
                ghp-import
                importlib-metadata
                jinja2
                markdown
                markupsafe
                mergedeep
                packaging
                pathspec
                platformdirs
                pyyaml
                pyyaml-env-tag
                watchdog
              ];
              doCheck = false;
            };

            # Every Go tool below is built the same way: no test suite (we are
            # packaging a release, not vetting it) and no symbol table or DWARF
            # (`-s -w`), which is about a third of a Go binary and dead weight
            # in something nobody debugs in place. `subPackages` narrows the
            # build to the commands the Makefile runs, wherever upstream ships
            # more — gogo/protobuf and code-generator ship a dozen each, buf
            # twenty-odd, and every one is tens of megabytes in the closure.
            goTool = args: buildGoModule ({
              doCheck = false;
              ldflags = [ "-s" "-w" ];
            } // args);
          in
          rec {
            # Used for `buf export` only, from hack/proto-export.
            buf = goTool rec {
              pname = "buf";
              version = toolVersions.buf;
              src = fetchFromGitHub {
                owner = "bufbuild";
                repo = "buf";
                rev = "v${version}";
                hash = "sha256-uhQhBxjtnWY5gm3vRv/SfLCdzW3cwMt91hHPUC25/O0=";
              };
              subPackages = [ "cmd/buf" ];
              vendorHash = "sha256-8Vh6txDsPOGad6rsW9hkahT+3Dku+aECaWpkGHgW7fs=";
            };

            # go-to-protobuf. The client, lister and informer generators live in
            # this repo too, but kube_codegen.sh `go install`s its own copies
            # into GOBIN and calls them by absolute path, so a copy on PATH
            # would only be shadowing that.
            code-generator = goTool rec {
              pname = "code-generator";
              version = toolVersions.codeGenerator;
              src = fetchFromGitHub {
                owner = "kubernetes";
                repo = "code-generator";
                rev = "v${version}";
                hash = "sha256-NhWD09Uy8QZLov74qhBmhqXGkxWalSjOMe/1He/fHns=";
              };
              subPackages = [ "cmd/go-to-protobuf" ];
              vendorHash = "sha256-eQuiQ8sCOE9wyVIBRmSQ1PkdvRIIw9I3GwSpHDPEE/I=";
            };

            controller-gen = final.kubernetes-controller-tools.overrideAttrs (old: rec {
              version = toolVersions.controllerTools;
              src = fetchFromGitHub {
                owner = "kubernetes-sigs";
                repo = "controller-tools";
                rev = "v${version}";
                hash = "sha256-zrh6GWFivs1fqkvaN6MSiYoCuPbiTQ6mJz4d69Wb7lo=";
              };
              vendorHash = "sha256-criu2UyNkGaVQnIxrjzIU4D389DbCcjG/kn3kfoD5yE=";
            });

            # protoc-gen-gogo and protoc-gen-gogofast.
            gogo-protobuf = goTool rec {
              pname = "gogo-protobuf";
              version = toolVersions.gogoProtobuf;
              src = fetchFromGitHub {
                owner = "gogo";
                repo = "protobuf";
                rev = "v${version}";
                hash = "sha256-CoUqgLFnLNCS9OxKFS7XwjE17SlH6iL1Kgv+0uEK2zU=";
              };
              subPackages = [ "protoc-gen-gogo" "protoc-gen-gogofast" ];
              vendorHash = "sha256-nOL2Ulo9VlOHAqJgZuHl7fGjz/WFAaWPdemplbQWcak=";
            };

            goimports = goTool rec {
              pname = "goimports";
              version = toolVersions.goimports;
              src = fetchFromGitHub {
                owner = "golang";
                repo = "tools";
                rev = "v${version}";
                hash = "sha256-h53fIvf3pedJXlopOEWYq5Hp7IVNsTIGychuCBPwY1I=";
              };
              subPackages = [ "cmd/goimports" ];
              vendorHash = "sha256-L2VYebgRTdiJyIBW437hvt8RyF4D4P8rjFvjNiDtu6Q=";
            };

            golangci-lint = final.golangci-lint.overrideAttrs (old: rec {
              version = toolVersions.golangciLint;
              src = fetchFromGitHub {
                owner = "golangci";
                repo = "golangci-lint";
                rev = "v${version}";
                hash = "sha256-VD46VOSBzVeeJ86FYLEPTsy23MUQapDPPYiO3/Ki8Mw=";
              };
              vendorHash = "sha256-k/lsDC6thW3B1zcn+OXjSmwmiW8pm0HM+g/z+N3AQek=";
            });

            go-swagger = final.go-swagger.overrideAttrs (old: rec {
              version = toolVersions.goSwagger;
              src = fetchFromGitHub {
                owner = "go-swagger";
                repo = "go-swagger";
                rev = "v${version}";
                hash = "sha256-CVfGKkqneNgSJZOptQrywCioSZwJP0XGspVM3S45Q/k=";
              };
              vendorHash = "sha256-x3fTIXmI5NnOKph1D84MHzf1Kod+WLYn1JtnWLr4x+U=";
            });

            gotestsum = goTool rec {
              pname = "gotestsum";
              version = toolVersions.gotestsum;
              src = fetchFromGitHub {
                owner = "gotestyourself";
                repo = "gotestsum";
                rev = "v${version}";
                hash = "sha256-j8lB0TIHK8/yMzaTB5OOaboEtnB6IsTybz8sJbNoQt4=";
              };
              subPackages = [ "." ];
              vendorHash = "sha256-UInHqKzntK0fYsUKZ2jW4akymeRd3sMQKf8+//TQb7g=";
            };

            # protoc-gen-grpc-gateway and protoc-gen-swagger.
            grpc-gateway = goTool rec {
              pname = "grpc-gateway";
              version = toolVersions.grpcGateway;
              src = fetchFromGitHub {
                owner = "grpc-ecosystem";
                repo = "grpc-gateway";
                rev = "v${version}";
                hash = "sha256-jJWqkMEBAJq50KaXccVpmgx/hwTdKgTtNkz8/xYO+Dc=";
              };
              vendorHash = "sha256-jVOb2uHjPley+K41pV+iMPNx67jtb75Rb/ENhw+ZMoM=";
            };

            kubeauto = goTool rec {
              pname = "kubeauto";
              version = toolVersions.kubeauto;
              src = fetchFromGitHub {
                owner = "kitproj";
                repo = "kubeauto";
                rev = "v${version}";
                hash = "sha256-WbGiTjxQBykwejx6iDctAZ53gwGgr2vAkK42kbQzkeE=";
              };
              vendorHash = "sha256-de5YVcBpU3tNpqilBwx68nuqBzU4e5ca/WNDPCsFPKA=";
            };

            mockery = final.go-mockery.overrideAttrs (old: rec {
              version = toolVersions.mockery;
              src = fetchFromGitHub {
                owner = "vektra";
                repo = "mockery";
                rev = "v${version}";
                hash = "sha256-x7WniZ4wpnuzUHM2ZC2P7Ns67bIp4V4F9f4xQEJONEk=";
              };
              vendorHash = "sha256-cNMknwlU7ENwN67CtyU1YgYIXCJbh4b7Z3oUK7kkEkk=";
              doCheck = false;
            });

            openapi-gen = goTool rec {
              pname = "openapi-gen";
              version = toolVersions.kubeOpenapi;
              src = fetchFromGitHub {
                owner = "kubernetes";
                repo = "kube-openapi";
                rev = "424119656bbf";
                hash = "sha256-rkI7r75euOv9c0QpGpLTfatFq5S3npynmKKNlflAHug=";
              };
              subPackages = [ "cmd/openapi-gen" ];
              vendorHash = "sha256-2PETLn3oDGIsyUQS7cY0XGTdMZvr7LCCc9fcltP0L80=";
            };

            # The Java SDK is generated by OpenAPI Generator 5.2.1 (see
            # sdks/java/Makefile), and the generated client is what downstream
            # users compile against, so the version is a contract. nixpkgs
            # builds a much newer one from source; take the published jar
            # instead and run it on a current JDK rather than the EOL JDK 8 it
            # was released against.
            openapi-generator-cli = final.stdenvNoCC.mkDerivation rec {
              pname = "openapi-generator-cli";
              version = toolVersions.openapiGenerator;
              jarfilename = "${pname}-${version}.jar";
              src = final.fetchurl {
                url = "mirror://maven/org/openapitools/${pname}/${version}/${jarfilename}";
                hash = "sha256-stRtSZCvPUQuTiKOHmJ7k8o3Gtly9Up+gicrDOeWjIs=";
              };
              dontUnpack = true;
              nativeBuildInputs = [ final.makeWrapper ];
              installPhase = ''
                runHook preInstall
                install -Dm644 $src $out/share/java/${jarfilename}
                makeWrapper ${final.jdk}/bin/java $out/bin/${pname} \
                  --add-flags "-jar $out/share/java/${jarfilename}"
                runHook postInstall
              '';
              meta = {
                description = "OpenAPI Generator CLI, pinned to the version the Java SDK is generated with";
                homepage = "https://openapi-generator.tech";
                license = lib.licenses.asl20;
                mainProgram = pname;
              };
            };

            inherit properdocs;

            # `properdocs` plus the theme and extensions it loads at runtime.
            pythonEnv = final.python3.withPackages (ps: [
              properdocs
              ps.mkdocs-material
              ps.mkdocs-redirects
              ps.pymdown-extensions
            ]);

            snipdoc = final.rustPlatform.buildRustPackage rec {
              pname = "snipdoc";
              version = toolVersions.snipdoc;
              src = fetchFromGitHub {
                owner = "kaplanelad";
                repo = "snipdoc";
                rev = "v${version}";
                hash = "sha256-3tF871gZouZMJ3LOzlucaxEy3q8TNoc08GVCT0aYOUk=";
              };
              cargoHash = "sha256-chi8q+zTewc7xpyvQbnMU7Lmd0Y4qFrIFCSh7IBITxU=";
              doCheck = false;
            };

            # cspell decides whether the docs spell-check passes, so its
            # version is a contract. nixpkgs happens to ship exactly the version
            # we want and packaging it by hand means vendoring a pnpm lockfile,
            # so assert on it instead: a nixpkgs bump that moves cspell fails
            # here rather than in a surprising docs job.
            cspell =
              let want = toolVersions.cspell; got = final.cspell.version; in
              if got == want then final.cspell
              else throw "cspell ${want} is pinned, but nixpkgs has ${got}: pin the new version in flake.nix after checking `make docs-spellcheck` still passes";
          };
      };

      pkgsFor = system: import nixpkgs {
        inherit system;
        overlays = [ goOverlay toolsOverlay ];
      };

      forAllSystems = f: lib.genAttrs systems (system: f (pkgsFor system));
    in
    {
      packages = forAllSystems (pkgs: pkgs.argoTools // { inherit (pkgs) go; });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          name = "argo-workflows";

          packages = with pkgs; [
            # -- pinned: codegen
            argoTools.code-generator
            argoTools.controller-gen
            argoTools.gogo-protobuf
            argoTools.goimports
            argoTools.go-swagger
            argoTools.grpc-gateway
            argoTools.mockery
            argoTools.buf
            argoTools.openapi-gen

            # -- pinned: lint, test and docs gates
            argoTools.cspell
            argoTools.golangci-lint
            argoTools.gotestsum
            argoTools.pythonEnv
            argoTools.snipdoc

            # -- pinned: SDKs
            argoTools.openapi-generator-cli
            jdk

            # -- pinned: not in nixpkgs at all
            argoTools.kubeauto

            # -- floating: language toolchains
            clang-tools # clang-format, for formatting .proto
            go
            gopls
            nodejs_24
            protobuf
            (yarn.override { nodejs = nodejs_24; })

            # -- floating: local cluster
            k3d
            kubectl
            kustomize
            tilt

            # -- floating: everything else
            diffutils
            gettext # envsubst, for the release notes
            gnused
            jq
            lsof
            markdown-link-check
            markdownlint-cli
            typos
          ];

          # Nix's Go sets its own GOROOT; a stale one inherited from the host
          # (a Homebrew or asdf install, say) makes it build against the wrong
          # standard library.
          shellHook = ''
            unset GOROOT
          '';
        };

        # The devcontainer CLI shells out to `docker`, so nixpkgs hands it a
        # whole engine — moby, containerd and buildx are half a gigabyte
        # between them. Only the CI job that bakes the image needs it, and
        # inside the container Docker comes from the docker-in-docker feature
        # anyway, so it gets a shell of its own instead of a place in the one
        # everybody enters:
        #
        #   nix develop .#devcontainer --command make devcontainer-build
        devcontainer = pkgs.mkShell {
          name = "argo-workflows-devcontainer";

          # The CLI wraps itself with git, docker and docker-compose on PATH.
          # Its docker already brings git-minimal along; point the wrapper at
          # that one too rather than dragging in a second, perl-laden git.
          packages = [
            (pkgs.devcontainer.override { git = pkgs.gitMinimal; })
          ];
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixpkgs-fmt);
    };
}
