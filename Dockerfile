# Build stage
FROM golang:1.20-alpine AS build
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN go build -o /livebox-exporter
# Final image
FROM gcr.io/distroless/static-debian11
WORKDIR /
COPY --from=build /livebox-exporter /usr/local/bin/livebox-exporter
EXPOSE 8080
USER 65534:65534
ENTRYPOINT ["livebox-exporter"]
