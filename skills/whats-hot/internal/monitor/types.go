// Package monitor samples system network activity from nettop and turns it into
// throughput-per-process and per-connection Snapshots. See docs/CONTEXT.md for the
// domain vocabulary (Throughput, Flow, Remote Endpoint, Snapshot).
package monitor

import "time"

// Endpoint is one side of a Flow: an address, a port, and (best-effort) a hostname.
// Host is filled by reverse DNS, or by TLS SNI in --deep mode. It is never a full URL.
type Endpoint struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
	Host string `json:"host,omitempty"`
}

// Flow is a single active network connection owned by one process.
type Flow struct {
	Proto     string   `json:"proto"` // tcp4 / tcp6 / udp4 / udp6
	Local     Endpoint `json:"local"`
	Remote    Endpoint `json:"remote"`
	Interface string   `json:"interface,omitempty"`
	State     string   `json:"state,omitempty"`
	BytesIn   int64    `json:"bytes_in"`  // cumulative, since nettop start
	BytesOut  int64    `json:"bytes_out"` // cumulative, since nettop start
	DownBps   int64    `json:"down_bps"`  // diffed throughput, bytes/sec
	UpBps     int64    `json:"up_bps"`    // diffed throughput, bytes/sec
	RTTms     float64  `json:"rtt_ms,omitempty"`
}

// Process aggregates all Flows owned by one PID.
type Process struct {
	Name     string `json:"name"`
	PID      int    `json:"pid"`
	BytesIn  int64  `json:"bytes_in"`
	BytesOut int64  `json:"bytes_out"`
	DownBps  int64  `json:"down_bps"`
	UpBps    int64  `json:"up_bps"`
	Flows    []Flow `json:"flows,omitempty"`
}

// Snapshot is one sample of all network activity: the machine total, broken down by
// process, each drillable to its Flows. This is the shape the snapshot command emits
// as JSON and the dashboard renders live.
type Snapshot struct {
	Time         time.Time `json:"time"`
	IntervalSec  float64   `json:"interval_sec"`
	TotalDownBps int64     `json:"total_down_bps"`
	TotalUpBps   int64     `json:"total_up_bps"`
	Processes    []Process `json:"processes"`
}
