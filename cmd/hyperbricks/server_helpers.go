package main

import (
	"net"

	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/staticserver"
)

func getConfig(requestedSlug string) (map[string]interface{}, bool) {
	// Lock for reading to ensure thread-safe access
	configMutex.RLock()
	defer configMutex.RUnlock()

	// Retrieve the map for the requestedSlug
	config, found := configs[requestedSlug]
	return config, found
}

func getConfigAndPlan(requestedSlug string) (map[string]interface{}, *renderplan.Plan, bool) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	config, found := configs[requestedSlug]
	return config, routePlans[requestedSlug], found
}

func getRenderSnapshot(route string) (map[string]interface{}, *renderplan.Plan, []error, uint64, bool) {
	configMutex.RLock()
	defer configMutex.RUnlock()
	config, found := configs[route]
	if !found {
		return nil, nil, getConfigSourceErrors(), routeGeneration, false
	}
	return config, routePlans[route], getRouteSourceErrors(route), routeGeneration, true
}

func getHostIPv4s() ([]string, error) {
	var ipAddresses []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			var ip net.IP

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.To4() == nil {
				continue
			}

			ipAddresses = append(ipAddresses, ip.String())
		}
	}

	return ipAddresses, nil
}

func normalizeRoutingConfig(routing shared.RoutingConfig) shared.RoutingConfig {
	return staticserver.NormalizeRouting(routing)
}
