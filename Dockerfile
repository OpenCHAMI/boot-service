# SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors
#
# SPDX-License-Identifier: MIT

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

ARG TARGETPLATFORM

# With GoReleaser dockers_v2, binaries are available under $TARGETPLATFORM/.
COPY $TARGETPLATFORM/boot-server /usr/local/bin/boot-server

USER nonroot:nonroot
EXPOSE 8080 9090

# Distroless runtime image: rely on external probes against /health rather than
# an in-container Docker HEALTHCHECK, because shell HTTP clients are not present.

ENTRYPOINT ["/usr/local/bin/boot-server"]
CMD ["serve"]
