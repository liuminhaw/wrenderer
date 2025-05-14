package localEnv

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	serverDefaultPort     = 8080
	serverDefaultKey      = "defaultKey"
	serverDefaultAdminKey = "adminKey"

	cacheDefaultEnabled         = true
	cacheDefaultType            = "boltdb"
	cacheDefaultPath            = "cache.db"
	cacheDefaultDuration        = 60
	cacheDefaultCleanupInterval = 60

	rendererDefaultWindowWidth  = 1920
	rendererDefaultWindowHeight = 1080
	rendererDefaultContainer    = false
	rendererDefaultHeadless     = true
	rendererDefaultUserAgent    = ""
	rendererDefaultTimeout      = 30
	rendererDefaultIdleType     = "auto"

	queueDefaultCapacity = 3
	queueDefaultWorkers  = 3

	semaphoreDefaultCapacity   = 5
	semaphoreDefaultJobTimeout = 60
)

func InitConfig() *viper.Viper {
	config := viper.New()
	config.SetConfigName("wrenderer")
	config.SetConfigType("toml")
	config.AddConfigPath(".")
	config.BindPFlags(pflag.CommandLine)

	return config
}

func ConfigSetup(config *viper.Viper) error {
	if err := config.ReadInConfig(); err != nil {
		return err
	}

	// Set default values
	config.SetDefault("app.port", serverDefaultPort)
	config.SetDefault("app.key", serverDefaultKey)
	config.SetDefault("app.adminKey", serverDefaultAdminKey)

	configureCache(config)
	configureRenderer(config)
	configureQueue(config)
	configureSemaphore(config)

	return nil
}

func configureCache(config *viper.Viper) {
	// Set default options
	config.SetDefault("cache.enabled", cacheDefaultEnabled)
	config.SetDefault("cache.type", cacheDefaultType)
	config.SetDefault("cache.path", cacheDefaultPath)
	config.SetDefault("cache.durationInMinutes", cacheDefaultDuration)
	config.SetDefault("cache.cleanupIntervalInMinutes", cacheDefaultCleanupInterval)

	config.Set("cache.enabled", config.GetBool("cache.enabled"))
	config.Set("cache.type", cacheDefaultType)
	if config.GetInt("cache.durationInMinutes") <= 0 {
		config.Set("cache.durationInMinutes", cacheDefaultDuration)
	}
	if config.GetInt("cache.cleanupIntervalInMinutes") <= 0 {
		config.Set("cache.cleanupIntervalInMinutes", cacheDefaultCleanupInterval)
	}
}

func configureRenderer(config *viper.Viper) {
	config.SetDefault("renderer.windowWidth", rendererDefaultWindowWidth)
	config.SetDefault("renderer.windowHeight", rendererDefaultWindowHeight)
	config.SetDefault("renderer.container", rendererDefaultContainer)
	config.SetDefault("renderer.headless", rendererDefaultHeadless)
	config.SetDefault("renderer.userAgent", rendererDefaultUserAgent)
	config.SetDefault("renderer.timeout", rendererDefaultTimeout)
	config.SetDefault("renderer.idleType", rendererDefaultIdleType)

	config.Set("renderer.container", config.GetBool("renderer.container"))
	config.Set("renderer.headless", config.GetBool("renderer.headless"))
	if config.GetInt("renderer.windowWidth") <= 0 {
		config.Set("renderer.windowWidth", rendererDefaultWindowWidth)
	}
	if config.GetInt("renderer.windowHeight") <= 0 {
		config.Set("renderer.windowHeight", rendererDefaultWindowHeight)
	}
	if config.GetInt("renderer.timeout") <= 0 {
		config.Set("renderer.timeout", rendererDefaultTimeout)
	}
	idleType := config.GetString("renderer.idleType")
	if idleType != "auto" && idleType != "networkIdle" && idleType != "InteractiveTime" {
		config.Set("renderer.idleType", rendererDefaultIdleType)
	}
}

func configureQueue(config *viper.Viper) {
	config.SetDefault("queue.capacity", queueDefaultCapacity)
	config.SetDefault("queue.workers", queueDefaultWorkers)

	if config.GetInt("queue.capacity") <= 0 {
		config.Set("queue.capacity", queueDefaultCapacity)
	}
	if config.GetInt("queue.workers") <= 0 {
		config.Set("queue.workers", queueDefaultWorkers)
	}
}

func configureSemaphore(config *viper.Viper) {
	config.SetDefault("semaphore.capacity", semaphoreDefaultCapacity)
	config.SetDefault("semaphore.jobTimeoutInMinutes", semaphoreDefaultJobTimeout)

	if config.GetInt("semaphore.capacity") <= 0 {
		config.Set("semaphore.capacity", semaphoreDefaultCapacity)
	}
	if config.GetInt("semaphore.jobTimeoutInMinutes") <= 0 {
		config.Set("semaphore.jobTimeoutInMinutes", semaphoreDefaultJobTimeout)
	}
}
