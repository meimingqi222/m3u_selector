package core

import (
	"time"
)

// M3U8Source represents a found M3U8 live stream source
type M3U8Source struct {
	URL           string
	Latency       time.Duration
	DownloadSpeed float64 // KB/s
	Valid         bool
	Error         string
	DataSize      int64 // bytes downloaded
	DownloadTime  time.Duration
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
