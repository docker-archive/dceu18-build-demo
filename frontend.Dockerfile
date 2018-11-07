# syntax = docker/dockerfile:experimental@sha256:d2d402b6fa1dae752f8c688d72066a912d7042cc1727213f7990cdb57f60df0c
FROM golang:1.11-alpine AS build

ADD . /go/src/dceu18-build-demo/
RUN --mount=target=/root/.cache,type=cache CGO_ENABLED=0 go build -o /frontend dceu18-build-demo/cli/frontend-container

FROM scratch
COPY --from=build /frontend /bin/frontend
ENTRYPOINT ["/bin/frontend"]
