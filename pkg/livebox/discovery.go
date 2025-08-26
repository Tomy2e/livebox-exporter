package livebox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
)

// Interface is a Livebox Network interface.
type Interface struct {
	Name  string
	Flags string
}

// IsWAN returns true if this interface is a WAN interface.
func (i *Interface) IsWAN() bool {
	return strings.Contains(i.Flags, "wan")
}

// IsWLAN returns true if this interface is a WLAN interface.
func (i *Interface) IsWLAN() bool {
	return strings.Contains(i.Flags, "wlanvap")
}

// DiscoverInterfaces discovers network interfaces on the Livebox.
func DiscoverInterfaces(ctx context.Context, client *livebox.Client) ([]*Interface, error) {
	var mibs struct {
		Status struct {
			Base map[string]struct {
				Flags string `json:"flags"`
			} `json:"base"`
		} `json:"status"`
	}

	if err := client.Request(
		ctx,
		request.New("NeMo.Intf.data", "getMIBs", map[string]any{
			"traverse": "all",
			// Only discover enabled interfaces: https://github.com/Tomy2e/livebox-exporter/issues/15
			"flag": "statmon && !vlan && enabled",
		}),
		&mibs,
	); err != nil {
		return nil, fmt.Errorf("failed to discover interfaces: %w", err)
	}

	if len(mibs.Status.Base) == 0 {
		return nil, errors.New("no interfaces found")
	}

	itfs := make([]*Interface, 0, len(mibs.Status.Base))

	for itf, val := range mibs.Status.Base {
		itfs = append(itfs, &Interface{
			Name:  itf,
			Flags: val.Flags,
		})
	}

	return itfs, nil
}
