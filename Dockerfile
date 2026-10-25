FROM gcr.io/distroless/static-debian13:nonroot
# Mount the directory with msi.yaml and its input files here.
WORKDIR /work
ARG TARGETPLATFORM
ENTRYPOINT ["/go-msi"]
COPY $TARGETPLATFORM/go-msi /
