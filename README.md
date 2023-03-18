# livebox-exporter

A prometheus exporter for Livebox. This exporter was tested with a Livebox 5 and
FTTH subscription.

## Metrics

This exporter currently exposes the following metrics:

| Name                               | Type  | Description                                       | Labels    | Experimental |
| ---------------------------------- | ----- | ------------------------------------------------- | --------- | ------------ |
| livebox_interface_rx_mbits         | gauge | Received Mbits per second                         | interface | No           |
| livebox_interface_tx_mbits         | gauge | Transmitted Mbits per second                      | interface | No           |
| livebox_devices_total              | gauge | The total number of active devices                | type      | No           |
| livebox_interface_homelan_rx_mbits | gauge | Received Mbits per second                         | interface | Yes          |
| livebox_interface_homelan_tx_mbits | gauge | Transmitted Mbits per second                      | interface | Yes          |
| livebox_interface_netdev_rx_mbits  | gauge | Received Mbits per second                         | interface | Yes          |
| livebox_interface_netdev_tx_mbits  | gauge | Transmitted Mbits per second                      | interface | Yes          |
| livebox_wan_rx_mbits               | gauge | Received Mbits per second on the WAN interface    | interface | Yes          |
| livebox_wan_tx_mbits               | gauge | Transmitted Mbits per second on the WAN interface | interface | Yes          |

Experimental metrics are not enabled by default, use the `-experimental`
command-line option to enable them.

## Usage

### Options

The exporter accepts the following command-line options:

| Name               | Description                                                                                                                                | Default value |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------ | ------------- |
| -polling-frequency | Polling frequency                                                                                                                          | 30            |
| -listen            | Listening address                                                                                                                          | :8080         |
| -experimental      | Comma separated list of experimental metrics to enable (available metrics: livebox_interface_homelan,livebox_interface_netdev,livebox_wan) |               |

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
