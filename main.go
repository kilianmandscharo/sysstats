package main

import (
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
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
var html string

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

func main() {
	v := unwrapError(mem.VirtualMemory())
	cpus := unwrapError(cpu.Percent(time.Second*1, true))
	for i := range len(cpus) {
		cpus[i] = round(cpus[i])
	}

	temps := unwrapError(sensors.SensorsTemperatures())
	hostInfo := unwrapError(host.Info())
	disk := unwrapError(disk.Usage("/"))

	funcMap := template.FuncMap{
		"inc": func(n int) int {
			return n + 1
		},
		"cpuColor": func(n float64) string {
			if n < 50 {
				return "green"
			}
			if n < 80 {
				return "orange"
			}
			return "red"
		},
	}

	stats := Stats{
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := template.Must(template.New("index").Funcs(funcMap).Parse(html))
		t.Execute(w, stats)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
