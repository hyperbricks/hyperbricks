package main

import (
	"net"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
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

func normalizeRoutingConfig(routing shared.RoutingConfig) shared.RoutingConfig {
	if len(routing.IndexFiles) == 0 {
		routing.IndexFiles = []string{"index.html", "index.htm"}
	}
	if len(routing.Extensions) == 0 {
		routing.Extensions = []string{"html", "htm"}
	}

	indexFiles := make([]string, 0, len(routing.IndexFiles))
	for _, name := range routing.IndexFiles {
		name = strings.TrimSpace(name)
		name = strings.TrimPrefix(name, "/")
		if name == "" {
			continue
		}
		indexFiles = append(indexFiles, name)
	}
	if len(indexFiles) == 0 {
		indexFiles = []string{"index.html"}
	}
	routing.IndexFiles = indexFiles

	extensions := make([]string, 0, len(routing.Extensions))
	for _, ext := range routing.Extensions {
		ext = strings.TrimSpace(ext)
		ext = strings.TrimPrefix(ext, ".")
		if ext == "" {
			continue
		}
		extensions = append(extensions, ext)
	}
	if len(extensions) == 0 {
		extensions = []string{"html"}
	}
	routing.Extensions = extensions

	return routing
}
