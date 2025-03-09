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
)

// htmlJsonTemplate is our dashboard HTML.
var htmlJsonTemplate string = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
  <title>Futuristic Dashboard</title>
  <script src="https://cdn.tailwindcss.com"></script>
  
  <style>
    @keyframes cycleColor {
      0%  { color: #479dc0; }
      50% { color: rgb(114, 187, 216); }
      100% { color: #479dc0; }
    }
    
    body {
      background-color: black;
      animation: cycleColor 20s linear infinite;
      font-family: monospace;
    }

    .border-glow {
      border: 1px solid currentColor;
      box-shadow: 0px 0px 2px currentColor;
    }
    h1, h2, p, span, a {
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
  </style>
</head>
<body class="p-4">
  <div class="max-w-md mx-auto border-glow p-4 rounded-lg">
        
  <div class="max-w-md mx-auto p-4 rounded-lg">
    <h1 class="text-center text-2xl font-bold">&lt;HyperBricks&gt; Dashboard</h1>
    <div class="mt-4 grid grid-cols-2 sm:grid-cols-3 gap-4 text-sm">
      <div class="border-glow p-2">
        <p>Memory Usage: <span id="memoryText">{{.Mem}}MiB</span></p>
      </div>
      <div class="border-glow p-2">
        <p>CPU Usage: <span id="cpuText">{{.Cpu}}</span></p>
        <div class="bar-container mt-1">
          <div id="cpuBar" class="bar-fill" style="width:{{.Cpu}};"></div>
        </div>
      </div>
    </div>
    <div class="border-glow p-2 mt-4 text-center flex items-center justify-center gap-2">
      Location: <span><a href="http://{{.Location}}">http://{{.Location}}</a></span>
    </div>
    <div class="border-glow p-2 mt-4 text-center flex items-center justify-center gap-2">
      <span>Current Module: <strong>{{.Module}}</strong></span>
    </div>
    <div class="mt-4">
      <h2 class="text-lg">Recent Visitor Activity</h2>
      <div class="activity-chart" id="activityChart"></div>
    </div>
    <div class="mt-4">
      <h2 class="text-lg">Routes</h2>
      {{range $key, $value := .Configs}}
        <div class="border-glow p-2 mt-2 flex justify-between">
          <span>/{{$value.route}}</span>
          <a href="#" class="underline">Config</a>
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
   </div> 
  <script>
    function generateActivityChart() {
      const chartContainer = document.getElementById("activityChart");
      chartContainer.innerHTML = "";
      const totalCols = 12;
      const totalRows = 10;
      const visitorCounts = Array.from({ length: totalCols }, () => Math.floor(Math.random() * (totalRows + 1)));
      for (let row = 0; row < totalRows; row++) {
        for (let col = 0; col < totalCols; col++) {
          const cell = document.createElement("div");
          cell.classList.add("activity-cell");
          if (row >= totalRows - visitorCounts[col]) {
            const dot = document.createElement("div");
            dot.classList.add("activity-dot");
            cell.appendChild(dot);
          }
          chartContainer.appendChild(cell);
        }
      }
    }
    
    generateActivityChart();

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
}

var (
	// Precompile the dashboard template at startup.
	tmpl = template.Must(template.New("table").Parse(htmlJsonTemplate))

	// cachedCPUUsage holds the latest CPU usage reading.
	cachedCPUUsage string = "0%"
)

// updateCPUUsage periodically updates cachedCPUUsage.
func updateCPUUsage() {
	// Create a ticker that fires every second.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// The blocking call here runs in the background and does not affect request handling.
		percent, err := cpu.Percent(time.Second, false)
		if err == nil && len(percent) > 0 {
			cachedCPUUsage = fmt.Sprintf("%d%%", int(math.Round(percent[0])))
		} else {
			cachedCPUUsage = fmt.Sprintf("%d%%", int(math.Round(50)))
		}
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

		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	go updateCPUUsage()
}
