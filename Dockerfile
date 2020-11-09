# build stage
FROM golang:latest AS build-env
ENV GO111MODULE=on

RUN go get -d github.com/GoogleCloudPlatform/k8s-node-termination-handler || true
WORKDIR /go/src/github.com/GoogleCloudPlatform/k8s-node-termination-handler

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -tags netgo -o node-termination-handler

# final stage
FROM gcr.io/distroless/static:latest
WORKDIR /app
COPY --from=build-env /go/src/github.com/GoogleCloudPlatform/k8s-node-termination-handler/node-termination-handler /app/
ENTRYPOINT ["./node-termination-handler"]
