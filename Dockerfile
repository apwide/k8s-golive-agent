#https://docs.docker.com/language/golang/build-images/
FROM golang:1.22-alpine AS build-stage

WORKDIR /app
COPY go.mod ./
COPY go.sum ./

RUN go mod download
COPY pkg ./pkg/
COPY cmd/k8s-golive-agent/main.go ./
COPY test ./test/

# RUN ls
ENV CGO_ENABLED=0
ENV GOOS=linux

RUN go test ./... -v && \
    go build -o /k8s-golive-agent
# RUN go build -o /app

FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /k8s-golive-agent /k8s-golive-agent

USER 1000:1000

# RUN chmod -R 755 /app
# CMD [ "/golive-k8s-tracker" ]
ENTRYPOINT ["/k8s-golive-agent"]
