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

const htmlJsonTemplate = `
<!DOCTYPE html>
        <html lang="en">
        <head>
          <meta charset="UTF-8" />
          <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
          <title>Futuristic Dashboard</title>
          <script src="https://cdn.tailwindcss.com"></script>
          
          <style>
            /* Animate through the specified colors on the <body> color */
            @keyframes cycleColor {
              0%  { color: #479dc0; }
              50% { color:rgb(114, 187, 216); }
              100%  { color: #479dc0; }
            }
            
            body {
              background-color: black;
              animation: cycleColor 20s linear infinite;
              font-family: monospace;
              /* The body's "color" is animated. Children that use currentColor will stay in sync. */
            }
        
            /* Everything below uses currentColor without extra transitions. */
            .border-glow {
              border: 1px solid currentColor;
              box-shadow: 0px 0px 2px currentColor;
            }
            h1, h2, p, span, a {
              color: currentColor;
            }
            
            /* CPU/Memory bar containers and fills both use currentColor */
            .bar-container { 
              width: 100%;
              height: 10px;
              border: 1px solid currentColor;
             position: relative;
            }
            .bar-fill {
              height: 100%;
             
              background: currentColor;
              opacity: 1; /* A bit of transparency for a subtle bar background */
              
            }
            
            /* Visitor Activity Chart */
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
          <div class="max-w-md mx-auto p-4 rounded-lg">
            <!-- Header -->
            <h1 class="text-center text-2xl font-bold">&lt;HyperBricks&gt; Dashboard</h1>
            
            <!-- System Stats -->
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
            <!-- Current Module -->
            <div class="border-glow p-2 mt-4 text-center flex items-center justify-center gap-2">
              <i class="fas fa-folder"></i>
              <span>Current Module: <strong>{{.Module}}</strong></span>
            </div>
            
            <!-- Recent Visitor Activity -->
            <div class="mt-4">
              <h2 class="text-lg">Recent Visitor Activity</h2>
              <div class="activity-chart" id="activityChart"></div>
            </div>
            
            <!-- Routes 
            <div class="mt-4">
              <h2 class="text-lg">Routes</h2>
              <div class="border-glow p-2 mt-2 flex justify-between">
                <span>/api/users</span>
                <span>12 clients</span>
                <a href="#" class="underline">Config</a>
              </div>
              <div class="border-glow p-2 mt-2 flex justify-between">
                <span>/api/products</span>
                <span>8 clients</span>
                <a href="#" class="underline">Config</a>
              </div>
              <div class="border-glow p-2 mt-2 flex justify-between">
                <span>/api/orders</span>
                <span>5 clients</span>
                <a href="#" class="underline">Config</a>
              </div>
            </div>-->

            <!-- Routes -->
            <div class="mt-4">
              <h2 class="text-lg">Routes</h2>
              {{range $key, $value := .Configs}}
                <div class="border-glow p-2 mt-2 flex justify-between">
                  <span>/{{$value.route}}</span>
                  
                  <a href="#" class="underline">Config</a>
                </div>
              {{end}}
            </div>
            
            <!-- Log Panel -->
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
            // Generate the 12×10 grid. Each column gets a random visitor count (0–10).
            // The bottom cells in each column (row >= totalRows - visitorCount) get a dot.
            function generateActivityChart() {
              const chartContainer = document.getElementById("activityChart");
              chartContainer.innerHTML = "";
              const totalCols = 12;
              const totalRows = 10;
              const visitorCounts = Array.from({ length: totalCols }, 
                                               () => Math.floor(Math.random() * (totalRows + 1)));
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
            setInterval(generateActivityChart, 10000);
          </script>
        </body>
        </html>`

// Convert bytes to megabytes.
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

type SysData struct {
	Cpu           string
	Mem           string
	Module        string
	Location      string
	Configs       map[string]map[string]interface{}
	CurrentConfig map[string]interface{}
	HbConfig      *shared.Config
}

func statusServer() {
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		var data SysData = SysData{}
		// Get memory statistics from the runtime.
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		// fmt.Printf("Memory Alloc = %v MiB\n", bToMb(m.Alloc))
		// fmt.Printf("Total Alloc = %v MiB\n", bToMb(m.TotalAlloc))
		// fmt.Printf("System Memory = %v MiB\n", bToMb(m.Sys))
		// fmt.Printf("Number of Garbage Collections = %v\n", m.NumGC)

		// Get CPU usage percentage over a 1-second interval.
		percent, err := cpu.Percent(time.Second, false)
		if err != nil {
			fmt.Println("Error getting CPU percent:", err)
			return
		}

		data.HbConfig = getHyperBricksConfiguration()

		configMutex.RLock()
		config := configs[r.URL.Path]
		configMutex.RUnlock()
		data.Module = shared.Module
		data.Location = shared.Location
		data.Configs = configs
		data.CurrentConfig = config
		data.Cpu = fmt.Sprintf("%d%%", int(math.Round(percent[0])))
		data.Mem = fmt.Sprintf("%d", int(bToMb(m.Sys)))
		// configMutex.RLock()
		// configtemp := configs
		// configMutex.RUnlock()
		// //var pagesCopy map[string]interface{}
		// var html strings.Builder
		// // Iterate over each section in the original map
		// for _, pages := range configtemp {
		// 	title, title_exists := pages["title"]
		// 	slug, slug_exists := pages["slug"]

		// 	if title_exists && slug_exists {
		// 		out := fmt.Sprintf(`<div><a href="/statusviewer/%s" target="%s" class="%s">%s</a></div>`, slug, "_self", "pages", title)

		// 		html.WriteString(out)
		// 	}

		// }

		tmpl := template.Must(template.New("table").Parse(htmlJsonTemplate))

		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
