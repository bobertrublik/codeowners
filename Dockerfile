# Get latest CA certs & git
FROM alpine:3.16 as deps

# hadolint ignore=DL3018
RUN apk --no-cache add ca-certificates git

FROM scratch

LABEL org.opencontainers.image.source=https://github.com/mszostok/codeowners

COPY ./codeowners /codeowners

COPY --from=deps /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=deps /usr/bin/git /usr/bin/git
COPY --from=deps /usr/bin/xargs  /usr/bin/xargs
COPY --from=deps /lib /lib
COPY --from=deps /usr/lib /usr/lib

ENTRYPOINT ["/codeowners"]
