# Build stage
FROM golang:1.17-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o /livebox-exporter
# Final image
FROM alpine:3.14
WORKDIR /
COPY --from=build /livebox-exporter /usr/local/bin/livebox-exporter
EXPOSE 8080
USER 10001:10001
ENTRYPOINT ["livebox-exporter"]
