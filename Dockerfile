#########################################################################################
# Build
#########################################################################################

# First stage: building the driver executable.
FROM docker.io/library/golang:1.22 as builder

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
COPY --from=builder --chown=65532:65532 /work/bin/s3gw-cosi-driver /usr/bin/s3gw-cosi-driver

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
ARG S3GW_VERSION=Development

# Add labels.
LABEL Name="s3gw-cosi-driver"
LABEL Version=${S3GW_VERSION}
LABEL description="COSI Driver for s3gw"
LABEL license="Apache-2.0"
LABEL maintainers="s3gw maintainers"
LABEL quay.expires-after=${QUAY_EXPIRATION}

# Set the entrypoint.
ENTRYPOINT [ "/usr/bin/3gw-cosi-driver" ]
CMD []
