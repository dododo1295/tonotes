package config

import (
	"main/utils"
	"time"
)

type DatabaseConfig struct {
    URI            string
    MaxPoolSize    uint64
    MinPoolSize    uint64
    MaxConnIdleTime time.Duration
    DatabaseName   string
    RetryWrites    bool
}

func LoadDatabaseConfig() DatabaseConfig {
    return DatabaseConfig{
        URI:            utils.GetEnvAsString("MONGO_URI", "mongodb://localhost:27017"),
        MaxPoolSize:    utils.GetEnvAsUint64("MONGO_MAX_POOL_SIZE", 100),
        MinPoolSize:    utils.GetEnvAsUint64("MONGO_MIN_POOL_SIZE", 10),
        MaxConnIdleTime: time.Duration(utils.GetEnvAsInt("MONGO_MAX_CONN_IDLE_TIME", 60)) * time.Second,
        DatabaseName:   utils.GetEnvAsString("MONGO_DB", "tonotes"),
        RetryWrites:    utils.GetEnvAsBool("MONGO_RETRY_WRITES", true),
    }
}
