FROM docker.io/library/golang:1.26.5-trixie@sha256:87ffdb09b6a2e29ff910748b745395e8a0299aa80b7c0551cdca9b55e3fd2b3e AS go-tools

ARG ACTIONLINT_VERSION=v1.7.12
ARG DEADCODE_VERSION=v0.48.0
ARG GOFUMPT_VERSION=v0.11.0
ARG GOLANGCI_LINT_VERSION=v2.12.2
ARG GOSEC_VERSION=v2.28.0
ARG GOPLS_VERSION=v0.23.0
ARG GOTESTSUM_VERSION=v1.13.0
ARG GOVULNCHECK_VERSION=v1.6.0
ARG STATICCHECK_VERSION=v0.7.0

RUN GOBIN=/opt/tools go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION} \
    && GOBIN=/opt/tools go install golang.org/x/tools/gopls@${GOPLS_VERSION} \
    && GOBIN=/opt/tools go install golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION} \
    && GOBIN=/opt/tools go install mvdan.cc/gofumpt@${GOFUMPT_VERSION} \
    && GOBIN=/opt/tools go install honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION} \
    && GOBIN=/opt/tools go install github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION} \
    && GOBIN=/opt/tools go install golang.org/x/tools/cmd/goimports@${DEADCODE_VERSION} \
    && GOBIN=/opt/tools go install golang.org/x/tools/cmd/deadcode@${DEADCODE_VERSION} \
    && GOBIN=/opt/tools go install gotest.tools/gotestsum@${GOTESTSUM_VERSION} \
    && GOBIN=/opt/tools go install github.com/rhysd/actionlint/cmd/actionlint@${ACTIONLINT_VERSION}

FROM docker.io/library/node:26.7.0-trixie-slim@sha256:298a542ccec8d9161ddac5427e05254838db3e23f0260c6a5bfb9278abbaaceb AS node-tools

ARG BEADS_VERSION=1.1.2
ARG CSPELL_VERSION=10.0.1
ARG MARKDOWNLINT_CLI2_VERSION=0.23.2
ARG NPM_VERSION=12.0.2

RUN npm install --global npm@${NPM_VERSION} \
    && npm install --global --allow-scripts=@beads/bd \
      @beads/bd@${BEADS_VERSION} \
      cspell@${CSPELL_VERSION} \
      markdownlint-cli2@${MARKDOWNLINT_CLI2_VERSION} \
    && npm cache clean --force

FROM docker.io/library/python:3.14.7-slim-trixie@sha256:83c1cebb322d099ac9e3a3a532ba74b0146d702838b25e4c75c02fa81ffeb910

ARG PATHSPEC_VERSION=1.1.1
ARG PYYAML_VERSION=6.0.3
ARG YAMLLINT_VERSION=1.38.0

LABEL org.opencontainers.image.source="https://github.com/ioplane/diskforge"

ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=0 \
    GOCACHE=/tmp/go-build \
    GOMODCACHE=/opt/go/pkg/mod \
    GOPATH=/opt/go \
    PATH=/usr/local/go/bin:/opt/go/bin:/usr/local/bin:/usr/bin:/bin

RUN apt-get update \
    && apt-get install --yes --no-install-recommends \
      ca-certificates \
      e2fsprogs \
      fdisk \
      file \
      gcc \
      git \
      libc6-dev \
      util-linux \
    && rm -rf /var/lib/apt/lists/* \
    && python -m pip install --no-cache-dir --root-user-action=ignore \
      pathspec==${PATHSPEC_VERSION} \
      PyYAML==${PYYAML_VERSION} \
      yamllint==${YAMLLINT_VERSION}

COPY --from=go-tools /usr/local/go /usr/local/go
COPY --from=go-tools /opt/tools/ /usr/local/bin/
COPY --from=node-tools /usr/local/bin/node /usr/local/bin/node
COPY --from=node-tools /usr/local/lib/node_modules/ /usr/local/lib/node_modules/
RUN ln -s ../lib/node_modules/@beads/bd/bin/bd.js /usr/local/bin/bd \
    && ln -s ../lib/node_modules/cspell/bin.mjs /usr/local/bin/cspell \
    && ln -s ../lib/node_modules/markdownlint-cli2/markdownlint-cli2-bin.mjs /usr/local/bin/markdownlint-cli2

WORKDIR /opt/diskforge/module
COPY go.mod go.sum ./
RUN go mod download \
    && chmod -R a+rX /opt/go \
    && git config --system --add safe.directory /workspace

WORKDIR /workspace
