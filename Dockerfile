FROM gcr.io/distroless/static:latest
LABEL maintainers="s3gw maintainers"
LABEL description="s3gw COSI driver"

COPY ./bin/s3gw-cosi-driver s3gw-cosi-driver
ENTRYPOINT ["/s3gw-cosi-driver"]
