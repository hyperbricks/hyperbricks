package main

import (
	"fmt"
	"html/template"
	"math"
	"mime"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/net"
)

// SysData holds system and configuration data for the dashboard.
type SysData struct {
	Cpu           string
	Mem           string
	Module        string
	Mode          string
	Gateway       string
	Port          string
	Configs       map[string]map[string]interface{}
	CurrentConfig map[string]interface{}
	HbConfig      *shared.Config
	Counter       string
	CacheExpire   string
	UpTime        string
	BandWidth     string
	Logs          []logging.LogMessage
	Plugins       map[string]shared.PluginRenderer
	PluginDir     string
}

var (
	// Precompile the dashboard template at startup.
	tmpl = template.Must(shared.GenericTemplate().Parse(assets.Dashboard))

	// cachedCPUUsage holds the latest CPU usage reading.
	cachedCPUUsage string = "0%"
	startTime      time.Time
	bandwidth      string
)

func MonitorBandwidth(interval time.Duration) string {
	prevStats, _ := net.IOCounters(false)
	time.Sleep(interval)
	currStats, _ := net.IOCounters(false)

	rxRate := float64(currStats[0].BytesRecv-prevStats[0].BytesRecv) / interval.Seconds()
	txRate := float64(currStats[0].BytesSent-prevStats[0].BytesSent) / interval.Seconds()

	return fmt.Sprintf(`D:%.2f KB/s U:%.2f KB/s`, rxRate/1024, txRate/1024)
}

// Uptime returns the duration since the application started as a formatted string
func Uptime() string {
	duration := time.Since(startTime)
	seconds := int(duration.Seconds())
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24
	weeks := days / 7
	months := days / 30 // Approximate
	years := days / 365 // Approximate

	seconds %= 60
	minutes %= 60
	hours %= 24
	days %= 7
	months %= 12

	var uptimeStr string
	if years > 0 {
		uptimeStr += fmt.Sprintf("%dy", years)
	}
	if months > 0 {
		uptimeStr += fmt.Sprintf("%dm", months)
	}
	if weeks > 0 {
		uptimeStr += fmt.Sprintf("%dw", weeks)
	}
	if days > 0 {
		uptimeStr += fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		uptimeStr += fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		uptimeStr += fmt.Sprintf("%dm", minutes)
	}
	if seconds > 0 || uptimeStr == "" {
		uptimeStr += fmt.Sprintf("%ds", seconds)
	}

	return uptimeStr
}

// updateCPUUsage periodically updates cachedCPUUsage.
func updateCPUUsage() {
	hbConfig := getHyperBricksConfiguration()
	startTime = time.Now()
	// Create a ticker that fires every second.
	ticker := time.NewTicker(hbConfig.System.MetricsWatchInterval)
	defer ticker.Stop()

	for range ticker.C {

		// The blocking call here runs in the background and does not affect request handling.
		percent, err := cpu.Percent(time.Second, false)
		if err == nil && len(percent) > 0 {
			cachedCPUUsage = fmt.Sprintf("%d%%", int(math.Round(percent[0])))
		} else {
			cachedCPUUsage = fmt.Sprintf("%d%%", int(math.Round(50)))
		}

		bandwidth = MonitorBandwidth(2 * time.Second)

	}

}
func stripAfterLastSlash(input string) string {
	if idx := strings.LastIndex(input, "/"); idx != -1 {
		return input[:idx]
	}
	return input
}
func stripPort(input string) string {
	if idx := strings.LastIndex(input, ":"); idx != -1 {
		return input[:idx]
	}
	return input
}

// bToMb converts bytes to megabytes.
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// statusServer registers the HTTP handler for the dashboard.
func statusServer() {
	hbConfig := getHyperBricksConfiguration()
	if !hbConfig.Development.Dashboard {
		return
	}

	//plugins = GetPlugins(hbConfig)
	http.HandleFunc("/assets/logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", mime.TypeByExtension(".png"))
		w.Write(assets.Logo)
	})

	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		var data SysData

		// Gather memory stats.
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		data.Mem = fmt.Sprintf("%d", int(bToMb(m.Sys)))

		// Use the cached CPU usage value.
		data.Cpu = cachedCPUUsage

		// Populate other data fields.
		data.Module = stripAfterLastSlash(shared.Module)
		data.Gateway = stripPort(shared.Location)

		// Assume 'configs' and 'configMutex' are defined elsewhere.
		data.Configs = configs
		configMutex.RLock()
		data.CurrentConfig = configs[r.URL.Path]
		configMutex.RUnlock()
		data.HbConfig = getHyperBricksConfiguration()
		data.CacheExpire = data.HbConfig.Live.CacheTime.String()
		data.Port = fmt.Sprintf("%d", data.HbConfig.Server.Port)
		data.Mode = data.HbConfig.Mode
		data.Counter = fmt.Sprintf("%d", requestCounter)
		data.UpTime = Uptime()
		data.BandWidth = bandwidth
		data.Logs = logging.GetLogs()
		data.Plugins = rm.Plugins

		pluginDir := "./bin/plugins"
		if tbplugindir, ok := rm.HbConfig.Directories["plugins"]; ok {
			pluginDir = tbplugindir
		}

		data.PluginDir = pluginDir

		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	go updateCPUUsage()

}
