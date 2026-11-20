# Jaeger's minimal runtime lacks wget; add a static BusyBox for its probe.
FROM busybox:1.37.0-musl AS probe
RUN mkdir -p /badger/data /badger/key
FROM jaegertracing/all-in-one:1.67.0
COPY --from=probe /bin/busybox /bin/busybox
COPY --from=probe --chown=10001:10001 /badger /badger
