# livebox-exporter

A prometheus exporter for Livebox. This exporter was tested with a Livebox 5 and
FTTH subscription.

## Metrics

This exporter currently exposes the following metrics:

| Name               | Type  | Description                  | Labels    |
| ------------------ | ----- | ---------------------------- | --------- |
| interface_rx_mbits | gauge | Received Mbits per second    | interface |
| interface_tx_mbits | gauge | Transmitted Mbits per second | interface |

## Usage

### Options

The exporter accepts the following command-line options:

| Name                | Description       | Default value |
| ------------------- | ----------------- | ------------- |
| --polling-frequency | Polling frequency | 30            |
| --listen            | Listening address | :8080         |

The exporter reads the following environment variables:

| Name           | Description                                                                                               | Default value |
| -------------- | --------------------------------------------------------------------------------------------------------- | ------------- |
| ADMIN_PASSWORD | Password of the Livebox "admin" user. The exporter will exit if this environment variable is not defined. |               |

### Docker

Use the following commands to run the exporter in Docker:

```console
docker build -t livebox-exporter .
docker run -p 8080:8080 -e ADMIN_PASSWORD=<changeme> livebox-exporter
```
