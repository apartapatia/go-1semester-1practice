package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	serverURL = "http://srv.msk01.gigacorp.local"
	interval  = 2 * time.Second
)

type Alert struct {
	Value     int
	Threshold int
	Message   string
	Triggered bool
}

func NewAlert(value int, threshold int, message string) *Alert {
	triggered := value > threshold

	return &Alert{
		Value:     value,
		Threshold: threshold,
		Message:   message,
		Triggered: triggered,
	}
}

func (a *Alert) Print() {
	if a.Triggered {
		formattedMessage := fmt.Sprintf(a.Message, a.Value)
		fmt.Println(formattedMessage)
	}
}

type Metrics struct {
	LoadAvg        int
	RAMTotal       int
	RAMUsed        int
	DiskTotal      int
	DiskUsed       int
	BandwidthTotal int
	BandwidthUsed  int
}

func parseMetrics(data string) (*Metrics, error) {
	parts := strings.Split(strings.TrimSpace(data), ",")
	if len(parts) != 7 {
		return nil, fmt.Errorf("invalid metrics data, expected 7 values, got %d", len(parts))
	}

	values := make([]int, 7)
	for i, s := range parts {
		v, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("invalid metric type: %w", err)
		}
		values[i] = v
	}

	const (
		valuesMB  = 1024 * 1024
		valuesMBs = 1000 * 1000
	)

	return &Metrics{
		LoadAvg:        values[0],
		RAMTotal:       values[1] / valuesMB,
		RAMUsed:        values[2] / valuesMB,
		DiskTotal:      values[3] / valuesMB,
		DiskUsed:       values[4] / valuesMB,
		BandwidthTotal: values[5] / valuesMBs,
		BandwidthUsed:  values[6] / valuesMBs,
	}, nil
}

func processAlerts(m *Metrics) {
	NewAlert(m.LoadAvg, 30, "Load Average is too high: %d").Print()
	NewAlert(m.RAMUsed*100/m.RAMTotal, 80, "Memory usage too high: %d%%").Print()
	NewAlert(m.DiskTotal-m.DiskUsed, m.DiskTotal/10, "Free disk space is too low: %d Mb left").Print()
	NewAlert(m.BandwidthTotal-m.BandwidthUsed, m.BandwidthTotal/10, "Network bandwidth usage high: %d Mbit/s available").Print()
}

type Monitoring struct {
	URL         string
	Client      *http.Client
	LastMetrics *Metrics
	ErrorCount  int
}

func (m *Monitoring) incrementErrorCount() {
	m.ErrorCount++

	if m.ErrorCount >= 3 {
		fmt.Println("Unable to fetch server statistic")
	}
}

func (m *Monitoring) resetErrorCount() {
	m.ErrorCount = 0
}

func (m *Monitoring) handleResponse(resp *http.Response) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Not 200 status code!")
		m.incrementErrorCount()
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Got read error %v\n", err)
		m.incrementErrorCount()
		return
	}

	metrics, err := parseMetrics(string(body))
	if err != nil {
		m.incrementErrorCount()
		return
	}

	m.LastMetrics = metrics
	m.resetErrorCount()
	processAlerts(metrics)
}

func main() {
	monitoring := &Monitoring{
		URL:    serverURL + "/_stats",
		Client: &http.Client{},
	}

	for {
		resp, err := monitoring.Client.Get(monitoring.URL)
		if err != nil {
			fmt.Printf("Got response error %v\n", err)
			monitoring.incrementErrorCount()
		} else {
			monitoring.handleResponse(resp)
		}

		time.Sleep(interval)
	}
}
