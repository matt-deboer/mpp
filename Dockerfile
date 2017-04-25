FROM scratch
COPY bin/mpp /mpp
ENTRYPOINT ["/mpp"]