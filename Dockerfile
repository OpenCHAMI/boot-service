FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# GoReleaser will build the binary and place it in the Docker build context
# as 'boot-service' for each target platform.
COPY boot-service /usr/local/bin/boot-service

# Include an example config for reference (not used by default runtime)
COPY config.example.yaml /etc/boot-service/config.example.yaml

USER nonroot:nonroot
EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/boot-service"]
CMD ["serve"]
