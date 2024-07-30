#########################################################################################
# Build
#########################################################################################

# First stage: building the driver executable.
FROM docker.io/library/golang:1.22.3 as builder

# Set the working directory.
WORKDIR /work

# Prepare dir so it can be copied over to runtime layer.
RUN mkdir -p /var/lib/cosi

# Copy the Go Modules manifests.
COPY go.mod go.mod
COPY go.sum go.sum

# Cache dep before building and copying source so that we don't need to re-download as
# much and so that source changes don't invalidate our downloaded layer.
RUN go mod download

# Copy the go source.
COPY Makefile Makefile
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build.
RUN make build

#########################################################################################
# Runtime
#########################################################################################

# Second stage: building final environment for running the executable.
FROM gcr.io/distroless/static:latest AS runtime

# Copy the executable.
COPY --from=builder --chown=65532:65532 /work/bin/seaweedfs-cosi-driver /usr/bin/seaweedfs-cosi-driver

# Copy the volume directory with correct permissions, so driver can bind a socket there.
COPY --from=builder --chown=65532:65532 /var/lib/cosi /var/lib/cosi

# Set volume mount point for app socket.
VOLUME [ "/var/lib/cosi" ]

# Set the final UID:GID to non-root user.
USER 65532:65532

# Disable healthcheck.
HEALTHCHECK NONE

# Few Args for dynamically setting labels.
ARG QUAY_EXPIRATION=Never
ARG SEAWEEDFS_VERSION=Development

# Add labels.

## Standard opencontainers labels.
LABEL org.opencontainers.image.title="seaweedfs-cosi-driver"
LABEL org.opencontainers.image.description="COSI Driver for seaweedfs"
LABEL org.opencontainers.image.authors="seaweedfs maintainers"
LABEL org.opencontainers.image.vendor="seaweedfs-tech"
LABEL org.opencontainers.image.version="${SEAWEEDFS_VERSION}"
LABEL org.opencontainers.image.license="Apache-2.0"
LABEL org.opencontainers.image.source="github.com/seaweedfs/seaweedfs-cosi-driver"
LABEL org.opencontainers.image.documentation="https://seaweedfs.github.io/"
LABEL org.opencontainers.image.base.name="gcr.io/distroless/static:latest"

## Quay specific labels.
LABEL quay.expires-after="${QUAY_EXPIRATION}"

# Set the entrypoint.
ENTRYPOINT [ "/usr/bin/seaweedfs-cosi-driver" ]
CMD []
