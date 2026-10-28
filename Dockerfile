FROM gcr.io/distroless/static-debian13:nonroot
# Mount the directory with msi.yaml and its input files here.
WORKDIR /work
ARG TARGETPLATFORM
ENTRYPOINT ["/msi-builder"]
COPY $TARGETPLATFORM/msi-builder /
