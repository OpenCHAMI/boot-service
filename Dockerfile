# SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors
#
# SPDX-License-Identifier: MIT

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

ARG TARGETPLATFORM

# With GoReleaser dockers_v2, binaries are available under $TARGETPLATFORM/.
COPY $TARGETPLATFORMboot-server /usr/local/bin/boot-server

USER nonroot:nonroot
EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/boot-server"]
CMD ["serve"]
