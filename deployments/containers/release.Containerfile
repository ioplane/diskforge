FROM ghcr.io/anchore/syft@sha256:1288ea4c8b38767b4e620c1e312c8cb26b6e887a99b4f07ab6cd19fc6f225026 AS syft

FROM ghcr.io/sigstore/cosign/cosign@sha256:9e5c2f2edc34351160407ca3416c61855bdf9403c3c5936e0f0be7fc261611b8 AS cosign

FROM ghcr.io/goreleaser/goreleaser@sha256:92b918cc587dce6321b5fafc57ba93942a38592a7fbdb6cc3e300418b9f03a7e

COPY --from=syft /syft /usr/bin/syft
COPY --from=cosign /ko-app/cosign /usr/bin/cosign

RUN goreleaser --version \
    && syft version \
    && cosign version

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

WORKDIR /workspace
ENTRYPOINT []
CMD ["goreleaser", "check"]
