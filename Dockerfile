#https://docs.docker.com/language/golang/build-images/
FROM golang:1.21-alpine AS build-stage

WORKDIR /app
COPY go.mod ./
COPY go.sum ./

RUN go mod download
COPY pkg ./pkg/
COPY cmd/k8s-monitor/main.go ./

# RUN ls

RUN CGO_ENABLED=0 GOOS=linux go build -o /k8s-golive-monitor
# RUN go build -o /app

FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /k8s-golive-monitor /k8s-golive-monitor

USER 1000:1000

# RUN chmod -R 755 /app
# CMD [ "/golive-k8s-tracker" ]
ENTRYPOINT ["/k8s-golive-monitor"]
