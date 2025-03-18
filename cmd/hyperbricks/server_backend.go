package main

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/net"
)

var newJsonTemplate string = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <script src="https://unpkg.com/@tailwindcss/browser@4"></script>
    <script src="https://unpkg.com/htmx.org@2.0.4"></script>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css">
    <meta name="generator" content="hyperbricks cms">
    <title>Dashboard</title>
  </head>
  <body class="bg-[#1F1F1F] text-gray-300 p-2 flex items-center justify-center min-h-screen mb-10">
    <div class="flex flex-col p-0 m-0">
      <div class="max-w-7xl mx-auto grid grid-cols-3 xs:grid-cols-2 gap-[3px] mb-[3px]">
        <section id="top_section" class="mb-[1px] mt-[4px] p-[4px] pb-[7px] col-span-3  bg-[#333] rounded-md">
          <h2 class="font-semibold text-[#05D000] text-sm mb-1 uppercase"></h2>
          <div class="flex flex-row justify-left items-center">
            <img style="width: 150px; height: 50px; object-fit: cover;" src="static/logo.png" />
            <h1 class="text-white w-96 text-center text-base font-semibold uppercase">
              Dashboard
            </h1>
            <div class="flex flex-row justify-left rounded mr-3 items-center bg-[#222] opacity-25">
              <div class="text-sm p-2">
                <span class="block sm:hidden">XS</span>
                <span class="hidden sm:block md:hidden">SM</span>
                <span class="hidden md:block lg:hidden">MD</span>
                <span class="hidden lg:block xl:hidden">LG</span>
                <span class="hidden xl:block 2xl:hidden">XL</span>
                <span class="hidden 2xl:block">2XL</span>
              </div>
            </div>
          </div>
        </section>
        
        <section id="general" class="p-2 pb-3 col-span-3 sm:col-span-1 md:col-span-1  bg-[#333] rounded-md">
          <h2 class="font-semibold text-[#05D000] text-sm mb-1 uppercase">Module</h2>
          <table class="w-full text-sm text-left">
            <thead class="text-gray-400 border-b border-[#222222]">
              <tr>
                <th class="p-1">Context</th>
                <th class="p-1">Value</th>
              </tr>
            </thead>
            <tbody>
              <tr class="border-b border-[#222222]"><td class="p-1 text-white">Module</td><td class="p-1 text-white">{{.Module}}</td></tr>
              <tr class="border-b border-[#222222]"><td class="p-1 text-white">Mode</td><td class="p-1 text-white">{{.Mode}}</td></tr>
              <tr class="border-b border-[#222222]"><td class="p-1 text-white">Gateway</td><td class="p-1 text-white">{{.Gateway}}</td></tr>
              <tr class="border-b border-[#222222]"><td class="p-1 text-white">Port</td><td class="p-1 text-white">{{.Port}}</td></tr>
              <tr><td class="p-1 text-white">Location</td><td class="p-1 text-white"><a href="http://{{.Gateway}}:{{.Port}}" class="text-[#05D000] underline"><i class="fas fa-external-link-alt"></i></a></td></tr>
            </tbody>
          </table>
        </section>
        
        <section id="metrics" class="p-2 pb-3 col-span-3 sm:col-span-2 md:col-span-2 bg-[#333] rounded-md">
          <h2 class="font-semibold text-[#05D000] text-sm mb-1 uppercase">Metrics</h2>
          <table class="w-full text-sm text-left">
            <thead class="text-gray-400 border-b border-[#222222]">
              <tr>
                <th class="p-1">Metric</th>
                <th class="p-1">Quantity</th>
              </tr>
            </thead>
            <tbody>
              <tr class="border-b border-[#222222]"><td class="p-1 text-white">Uptime</td><td class="p-1 text-white">{{.UpTime}}</td></tr>
              <tr class="border-b border-[#222222]"><td class="p-1 text-white">Total Requests</td><td class="p-1 text-white">{{.Counter}}</td></tr>
              <tr class="border-b border-[#222222]"><td class="p-1 text-white">Default Cache Expire</td><td class="p-1 text-white">{{.CacheExpire}}</td></tr>
              <tr class="border-b border-[#222222]"><td class="p-1 text-white">Memory</td><td class="p-1 text-white">{{.Mem}} MiB</td></tr>
              <tr class="border-b border-[#222222]"><td class="p-1 text-white">CPU Usage</td><td class="p-1 text-white">{{.Cpu}}</td></tr>
              <tr><td class="p-1 text-white">Bandwidth</td><td class="p-1 text-white">{{.BandWidth}}</td></tr>
            </tbody>
          </table>
        </section>
        
        <section id="routes" class="p-2 pb-3 col-span-3 bg-[#333] rounded-md">
          <h2 class="font-semibold text-[#05D000] text-sm mb-1 uppercase">Routes</h2>
          <table class="w-full text-sm text-left">
            <thead class="text-gray-400 border-b border-[#222222]">
              <tr>
                <th class="p-1">Location</th>
                <th class="p-1">Route Type</th>
                <th class="p-1">Link</th>
                <th class="p-1 opacity-25">Config</th>
              </tr>
            </thead>
            <tbody>
              {{range $key, $value := .Configs}}
              <tr class="border-b border-[#222222]">
                <td class="p-1 text-white">/{{$value.route}}</td>
                <td class="p-1 text-white">{{$value.type}}</td>
                <td class="p-1 text-white"><a href="/{{$value.route}}" class="text-[#05D000] underline"><i class="fas fa-external-link-alt"></i></a></td>
                <td class="p-1 text-white"><a href="/{{$value.route}}" class="text-[#05D000] underline text-center opacity-25"><i class="fas fa-cog"></i></a></td>
              </tr>
              {{end}}
            </tbody>
          </table>
        </section> 
        <section id="logs" class="p-2 pb-3 col-span-3  bg-[#333] rounded-md opacity-25">
          <h2 class="font-semibold text-[#05D000] text-sm mb-1 uppercase"></h2>
          <div class="flex justify-between items-center">
            <h2 class="font-semibold text-[#05D000] text-sm uppercase">
              Logs
            </h2>
            <button id="toggleLogs" class="text-[#05D000] text-lg">
              <span id="logsPlusIcon">
                +
              </span>
              <span id="logsMinusIcon" class="hidden">
                −
              </span>
            </button>
          </div>
          <div id="logsContainer" class="hidden">
            <table class="w-full text-sm text-left">
              <thead class="text-gray-400 border-b border-[#222222]">
                <tr>
                  <th class="p-1">
                    Level
                  </th>
                  <th class="p-1">
                    Message
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr class="border-b border-[#222222]">
                  <td class="p-1 text-white">
                    INFO
                  </td>
                  <td class="p-1 text-white">
                    Loading hyperbricks files in {{.Module}}
                  </td>
                </tr>
                <tr class="border-b border-[#222222]">
                  <td class="p-1 text-white">
                    INFO
                  </td>
                  <td class="p-1 text-white">
                    Preprocessing hyperbricks configurations...
                  </td>
                </tr>
              </tbody>
            </table>
            <script>
              document.getElementById('toggleLogs').addEventListener('click', function () {
              const logs = document.getElementById('logsContainer');
              const plusIcon = document.getElementById('logsPlusIcon');
              const minusIcon = document.getElementById('logsMinusIcon');

              if (logs.classList.contains('hidden')) {
              logs.classList.remove('hidden');
              plusIcon.classList.add('hidden');
              minusIcon.classList.remove('hidden');
              } else {
              logs.classList.add('hidden');
              plusIcon.classList.remove('hidden');
              minusIcon.classList.add('hidden');
              }
              });
            </script>
          </div>
        </section>
        <section id="plugins" class="p-2 pb-3 col-span-3  bg-[#333] rounded-md opacity-25">
          <h2 class="font-semibold text-[#05D000] text-sm mb-1 uppercase">
            plugins
          </h2>
          <table class="w-full text-sm text-left">
            <thead class="text-gray-400 border-b border-[#222222]">
              <tr>
                <th class="p-1">
                  Context
                </th>
                <th class="p-1">
                  Value
                </th>
              </tr>
            </thead>
            <tr class="border-b border-[#222222]">
              <td class="p-1 text-white">
                Plugin Dir
              </td>
              <td class="p-1 text-white">
                ./bin/plugins
              </td>
            </tr>
            <thead class="text-gray-400 border-b border-[#222222] ">
              <tr>
                <th class="p-1">
                  Plugin
                </th>
                <th class="p-1">
                  Key
                </th>
                <th class="p-1">
                  Location
                </th>
              </tr>
            </thead>
            <tbody>
              <tr class="border-b border-[#222222]">
                <td class="p-1 text-white">
                  Lorem Ipsum
                </td>
                <td class="p-1 text-white">
                  lorem_ipsum_plugin
                </td>
                <td class="p-1 text-white">
                  /lorem_ipsum_plugin
                </td>
              </tr>
              <tr class="border-b border-[#222222]">
                <td class="p-1 text-white">
                  Blog
                </td>
                <td class="p-1 text-white">
                  blog_plugin
                </td>
                <td class="p-1 text-white">
                  /blog_plugin
                </td>
              </tr>
            </tbody>
          </table>
        </section>
      </div>
      <section id="timeline" class="p-2 pb-3 col-span-3 sm:col-span-1 md:col-span-1  bg-[#333] rounded-md opacity-25">
        <h2 class="font-semibold text-[#05D000] text-sm mb-1 uppercase"></h2>
        <div class="relative">
          <div class="flex justify-between items-center">
            <h2 class="font-semibold text-[#05D000] text-sm uppercase">
              Timeline
            </h2>
            <button id="toggleTimeline" class="text-[#05D000] text-lg">
              <span id="plusIcon">
                +
              </span>
              <span id="minusIcon" class="hidden">
                −
              </span>
            </button>
          </div>
          <div id="timelineContainer" class="container hidden">
            <div class="flex flex-col md:grid grid-cols-12 text-gray-50">
              <div class="flex md:contents">
                <div class="col-start-2 col-end-4 mr-6 md:mx-auto relative">
                  <div class="h-full w-4 flex items-center justify-center">
                    <div class="h-full w-1 bg-green-500 pointer-events-none rounded-t-full"></div>
                  </div>
                  <div class="w-5 h-5 absolute top-1/2 -mt-2.5 -ml-0.5 rounded-full bg-green-500 shadow flex items-center justify-center"></div>
                </div>
                <div class="bg-green-500 col-start-4 col-end-12 p-3 rounded-lg my-2 mr-auto shadow-md w-full">
                  <h3 class="font-semibold text-base mb-1">
                    HyperBricks Started
                  </h3>
                  <p class="leading-tight text-justify w-full">
                    21 July 2021, 01:30 PM
                  </p>
                </div>
              </div>
              <div class="flex md:contents">
                <div class="col-start-2 col-end-4 mr-6 md:mx-auto relative">
                  <div class="h-full w-4 flex items-center justify-center">
                    <div class="h-full w-1 bg-blue-500 pointer-events-none rounded-t-full"></div>
                  </div>
                  <div class="w-5 h-5 absolute top-1/2 -mt-2.5 -ml-0.5 rounded-full bg-blue-500 shadow flex items-center justify-center"></div>
                </div>
                <div class="bg-blue-500 col-start-4 col-end-12 p-3 rounded-lg my-2 mr-auto shadow-md w-full">
                  <h3 class="font-semibold text-base mb-1">
                    HyperBricks Stopped
                  </h3>
                  <p class="leading-tight text-justify w-full">
                    21 July 2021, 01:345 PM
                  </p>
                </div>
              </div>
              <div class="flex md:contents">
                <div class="col-start-2 col-end-4 mr-6 md:mx-auto relative">
                  <div class="h-full w-4 flex items-center justify-center">
                    <div class="h-full w-1 bg-red-500 pointer-events-none rounded-t-full"></div>
                  </div>
                  <div class="w-5 h-5 absolute top-1/2 -mt-2.5 -ml-0.5 rounded-full bg-red-500 shadow flex items-center justify-center"></div>
                </div>
                <div class="bg-red-500 col-start-4 col-end-12 p-3 rounded-lg my-2 mr-auto shadow-md w-full">
                  <h3 class="font-semibold text-base mb-1">
                    HyperBricks Error
                  </h3>
                  <p class="leading-tight text-justify w-full">
                    21 July 2021, 02:00 PM
                  </p>
                </div>
              </div>
              <div class="flex md:contents">
                <div class="col-start-2 col-end-4 mr-6 md:mx-auto relative">
                  <div class="h-full w-4 flex items-center justify-center">
                    <div class="h-full w-1 bg-green-500 pointer-events-none rounded-t-full"></div>
                  </div>
                  <div class="w-5 h-5 absolute top-1/2 -mt-2.5 -ml-0.5 rounded-full bg-green-500 shadow flex items-center justify-center"></div>
                </div>
                <div class="bg-green-500 col-start-4 col-end-12 p-3 rounded-lg my-2 mr-auto shadow-md w-full">
                  <h3 class="font-semibold text-base mb-1">
                    HyperBricks Started
                  </h3>
                  <p class="leading-tight text-justify w-full">
                    21 July 2021, 02:30 PM
                  </p>
                </div>
              </div>
            </div>
          </div>
          <script>
            document.getElementById('toggleTimeline').addEventListener('click', function () {
            const timeline = document.getElementById('timelineContainer');
            const plusIcon = document.getElementById('plusIcon');
            const minusIcon = document.getElementById('minusIcon');

            if (timeline.classList.contains('hidden')) {
            timeline.classList.remove('hidden');
            plusIcon.classList.add('hidden');
            minusIcon.classList.remove('hidden');
            } else {
            timeline.classList.add('hidden');
            plusIcon.classList.remove('hidden');
            minusIcon.classList.add('hidden');
            }
            });
          </script>
        </div>
      </section>
    </div>
  </body>
</html>`

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
}

var (
	// Precompile the dashboard template at startup.
	tmpl = template.Must(template.New("table").Parse(newJsonTemplate))

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
