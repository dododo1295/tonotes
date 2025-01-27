package utils

import (
	"sync/atomic"
	"time"
)

type MongoMetrics struct {
    ActiveConnections   int64
    AvailableConnections int64
    CreatedConnections   int64
    ClosedConnections    int64
    LastCheckTime       time.Time
}

var metrics MongoMetrics

func IncrementActiveConnections() {
    atomic.AddInt64(&metrics.ActiveConnections, 1)
}

func DecrementActiveConnections() {
    atomic.AddInt64(&metrics.ActiveConnections, -1)
}

func GetMongoMetrics() MongoMetrics {
    return metrics
}
