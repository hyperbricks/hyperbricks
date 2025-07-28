package main

import (
	"net"
)

func getConfig(requestedSlug string) (map[string]interface{}, bool) {
	// Lock for reading to ensure thread-safe access
	configMutex.RLock()
	defer configMutex.RUnlock()

	// Retrieve the map for the requestedSlug
	config, found := configs[requestedSlug]
	return config, found
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
