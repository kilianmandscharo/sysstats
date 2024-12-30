package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/sensors"
)

func unwrapError[T any](val T, err error) T {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s", err)
		os.Exit(1)
	}
	return val
}

func toGb(num uint64) float64 {
	return round(float64(num) / (1024 * 1024 * 1024))
}

func round(num float64) float64 {
	return math.Round(num*100) / 100
}

func formatDuration(seconds uint64) string {
	hours := seconds / 3600
	seconds = seconds % 3600
	minutes := seconds / 60
	seconds = seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

//go:embed index.html
var indexHtml string

//go:embed stats.html
var statsHtml string

type Stats struct {
	TotalDisk        float64
	UsedDisk         float64
	PercentageDisk   float64
	TotalMemory      float64
	UsedMemory       float64
	PercentageMemory float64
	Cpus             []float64
	Hostname         string
	Uptime           string
	Os               string
	Temps            []sensors.TemperatureStat
}

func getStats() *Stats {
	v := unwrapError(mem.VirtualMemory())
	cpus := unwrapError(cpu.Percent(time.Second*1, true))
	for i := range len(cpus) {
		cpus[i] = round(cpus[i])
	}
	temps := unwrapError(sensors.SensorsTemperatures())
	hostInfo := unwrapError(host.Info())
	disk := unwrapError(disk.Usage("/"))
	return &Stats{
		TotalDisk:        toGb(disk.Total),
		UsedDisk:         toGb(disk.Used),
		PercentageDisk:   round(disk.UsedPercent),
		UsedMemory:       toGb(v.Used),
		TotalMemory:      toGb(v.Total),
		PercentageMemory: round(v.UsedPercent),
		Cpus:             cpus,
		Hostname:         hostInfo.Hostname,
		Uptime:           formatDuration(hostInfo.Uptime),
		Os:               hostInfo.OS,
		Temps:            temps,
	}
}

func main() {
	funcMap := template.FuncMap{
		"inc": func(n int) int {
			return n + 1
		},
		"cpuColor": func(n float64) string {
			if n <= 50 {
				return "green"
			}
			if n <= 90 {
				return "orange"
			}
			return "red"
		},
		"temperatureColor": func(n float64) string {
			if n <= 50 {
				return "green"
			}
			if n <= 90 {
				return "orange"
			}
			return "red"
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := template.Must(template.New("index").Parse(indexHtml))
		t.Execute(w, "")
	})

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		for {
			t := template.Must(template.New("stats").Funcs(funcMap).Parse(statsHtml))
			var buf bytes.Buffer
			t.Execute(&buf, *getStats())
			fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(buf.String(), "\n", ""))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(1 * time.Second)
		}
	})

	log.Println("Listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
