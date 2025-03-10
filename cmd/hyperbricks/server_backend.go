package main

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"runtime"
	"time"

	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/net"
)

// htmlJsonTemplate is our dashboard HTML.
var htmlJsonTemplate string = `<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>&lt;HyperBricks&gt; Dashboard</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css">

    <style>
        body {
            color: #387d98; 
            background-color:rgb(8, 8, 8); 
            animation: cycleColor 20s linear infinite;
            font-family: monospace;
        }

        .border-glow {
            background-color:rgb(8, 8, 8); 
            border-left: 1px solid rgb(15, 35, 42); 
            border-right: 1px solid rgb(15, 35, 42); 
            border-top: 1px solid rgb(15, 35, 42); 
            border-bottom: 4px solid #183540; 

            border: 4px solid #183540; 
           
        }

        .labels {
            color: #5ac9f5; 
        }

        h1,
        h2,
        p,
        span,
        a {
            color: currentColor;
        }

        .bar-container {
            width: 100%;
            height: 10px;
            border: 1px solid currentColor;
            position: relative;
        }

        .bar-fill {
            height: 100%;
            background: currentColor;
            opacity: 1;
        }

        .activity-chart {
            display: grid;
            grid-template-columns: repeat(12, 1fr);
            grid-template-rows: repeat(10, 1fr);
            width: 100%;
            height: 200px;
            border: 1px solid currentColor;
            box-sizing: border-box;
        }

        .activity-cell {
            border: .1px solid currentColor;
            box-sizing: border-box;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .activity-dot {
            width: 10px;
            height: 10px;
            border-radius: 50%;
            background-color: currentColor;
        }

        
        .status-dot {
            width: 10px;
            height: 10px;
            border-radius: 50%;
        }

        .status-ok {
            background-color: hsl(186, 58%, 66%);
        }

        .status-warn {
            background-color: hsl(186, 58%, 66%);
        }

        .status-error {
            background-color: hsl(186, 58%, 66%);
        }
    </style>
</head>

<body class="m-2 mt-5 p-0">
    <div class="max-w-3xl mx-auto p-2 ">
            <h1 class="text-center text-2xl font-bold"><span class="labels">&lt;HyperBricks&gt;</span> Dashboard</h1>
            <div class="border-glow p-2 mt-4 text-center flex items-center justify-center gap-2">
                Location: <span><a class="labels" href="{{.Location}}">{{.Location}}</a></span>
            </div>
            <div class="border-glow p-2 mt-4 text-center flex items-center justify-center gap-2">
                <span>Module Running: <strong class="labels">{{.Module}}</strong></span>
            </div>
            <div class="border-glow p-2 mb-2 mt-4 text-center flex items-center justify-center gap-2">
                <p><span class="labels" id="cpuText">DEVELOPMENT MODE</span></p>
            </div>
            <div class="border-glow p-2 mt-4 text-center flex items-center justify-center gap-2">
                Documentation: <span><a class="labels" target="docs" href="https://github.com/hyperbricks/hyperbricks/blob/main/README.md">LINK</a></span>
            </div>
             <div class="mt-4">
                <h2 class="text-lg">Metrics</h2>
                        <div class="mt-4 grid grid-cols-3 sm:grid-cols-3 gap-4 text-sm">
                <div class="border-glow p-2 rounded-sm flex items-center justify-center text-center">
                    <p>Uptime<span id="upTimeText" class="labels"><br>{{.UpTime}}</span></p>
                </div>
                <div class="border-glow p-2 rounded-sm flex items-center justify-center text-center">
                    <p>Total requests<span class="labels" id="memoryText"><br>{{.Counter}}</span></p>
                </div>
                <div class="border-glow p-2 rounded-sm flex items-center justify-center text-center">
                    <p>Cache expire<span class="labels" id="cpuText"><br>{{.CacheExpire}}</span></p>
                </div>
            </div>

            <div class="mt-4 grid grid-cols-3 sm:grid-cols-3 gap-4 text-sm">
                <div class="border-glow p-2 rounded-sm flex items-center justify-center text-center">
                    <p>Memory<span id="memoryText" class="labels"><br>{{.Mem}}MiB</span></p>
                </div>
                <div class="border-glow p-2 rounded-sm  text-center">
                    <p>CPU Usage<span id="cpuText"><br>{{.Cpu}}</span></p>
                    <div class="bar-container mt-1">
                        <div id="cpuBar" class="bar-fill labels" style="width: {{.Cpu}};"></div>
                    </div>
                </div>
                <div class="border-glow p-2 rounded-sm flex items-center justify-center text-center">
                    <p>Bandwidth<span id="cpuText" class="labels"><br>{{.BandWidth}}</span></p>
                </div>
            </div>
             </div>

            

            <!-- <div class="mt-4">
                <h2 class="text-lg">Recent Visitor Activity</h2>
                <div class="activity-chart" id="activityChart"></div>
            </div> -->
           
            <div class="mt-4">
                <h2 class="text-lg">Routes</h2>
                {{range $key, $value := .Configs}}
                    <div class="border-glow p-2 mt-2 flex justify-between items-center">
                        <div class="flex items-center gap-3">
                            <div class="status-dot status-warn"></div>
                            <span class="labels">/{{$value.route}}</span>
                        </div>
                        <div class="flex items-center gap-3">
                            <a href="/{{$value.route}}"><i class="fas fa-external-link-alt"></i></a>
                            <a href="/{{$value.route}}"><i class="fas fa-cog"></i></a>
                        </div>
                    </div>
                {{end}}
              </div>
            <div class="mt-4 border-glow p-2 relative">
                <button onclick="document.getElementById('logPanel').classList.toggle('hidden')"
                    class="absolute top-1 right-1 text-xs">[+]</button>
                <h2 class="text-lg">Logs</h2>
                <div id="logPanel" class="hidden">
                    <p>[INFO] Server started</p>
                    <p>[ERROR] Connection timeout</p>
                    <p>[INFO] Client connected: 192.168.1.42</p>
                </div>
            </div>
        </div>
   
    <script>
        

        // This function refreshes the page
        function refreshPage() {
            window.location.reload();
        }

        // Refresh the page after 10 seconds (10,000 milliseconds)
        setTimeout(refreshPage, 10000);

        
    </script>
</body>

</html>`

// SysData holds system and configuration data for the dashboard.
type SysData struct {
	Cpu           string
	Mem           string
	Module        string
	Location      string
	Configs       map[string]map[string]interface{}
	CurrentConfig map[string]interface{}
	HbConfig      *shared.Config
	Counter       string
	CacheExpire   string
	UpTime        string
	BandWidth     string
}

var (
	// Precompile the dashboard template at startup.
	tmpl = template.Must(template.New("table").Parse(htmlJsonTemplate))

	// cachedCPUUsage holds the latest CPU usage reading.
	cachedCPUUsage string = "0%"
	counter        string = "0"
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
	startTime = time.Now()
	// Create a ticker that fires every second.
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for range ticker.C {

		// The blocking call here runs in the background and does not affect request handling.
		percent, err := cpu.Percent(time.Second, false)
		if err == nil && len(percent) > 0 {
			cachedCPUUsage = fmt.Sprintf("%d%%", int(math.Round(percent[0])))
		} else {
			cachedCPUUsage = fmt.Sprintf("%d%%", int(math.Round(50)))
		}

		counter = fmt.Sprintf("%d", requestCounter)

		bandwidth = MonitorBandwidth(2 * time.Second)

	}

}

// bToMb converts bytes to megabytes.
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// statusServer registers the HTTP handler for the dashboard.
func statusServer() {
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		var data SysData

		// Gather memory stats.
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		data.Mem = fmt.Sprintf("%d", int(bToMb(m.Sys)))

		// Use the cached CPU usage value.
		data.Cpu = cachedCPUUsage

		// Populate other data fields.
		data.Module = shared.Module
		data.Location = shared.Location
		// Assume 'configs' and 'configMutex' are defined elsewhere.
		data.Configs = configs
		configMutex.RLock()
		data.CurrentConfig = configs[r.URL.Path]
		configMutex.RUnlock()
		data.HbConfig = getHyperBricksConfiguration()
		data.CacheExpire = data.HbConfig.Live.CacheTime.String()
		data.Counter = counter
		data.UpTime = Uptime()
		data.BandWidth = bandwidth

		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	go updateCPUUsage()

}
