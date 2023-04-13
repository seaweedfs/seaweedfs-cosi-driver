FROM gcr.io/distroless/static:latest
LABEL Name="s3gw-cosi-driver"
LABEL maintainers="s3gw maintainers"
LABEL description="s3gw COSI driver"

ARG QUAY_EXPIRATION=Never
ARG S3GW_VERSION=Development
ARG ID=s3gw-cosi-driver

ENV ID=${ID}

LABEL Version=${S3GW_VERSION}
LABEL quay.expires-after=${QUAY_EXPIRATION}

COPY ./bin/s3gw-cosi-driver s3gw-cosi-driver
ENTRYPOINT ["/s3gw-cosi-driver"]
