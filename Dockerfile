FROM scratch

ARG BIN_PATH=dora-the-explorer

ARG UID=10001
USER ${UID}

COPY --chmod=755 ${BIN_PATH} /usr/bin/dora-the-explorer


ENTRYPOINT ["/usr/bin/dora-the-explorer"]
