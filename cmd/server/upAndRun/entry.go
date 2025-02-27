package upAndRun

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/wrenderer/cmd/worker/upAndRunWorker"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func Start() error {
	pflag.Bool("debug", false, "Enable debug mode")
	pflag.Bool(
		"chromiumDebug",
		false,
		"Enable chromium debug mode, enable this will automatically enable debug mode",
	)
	pflag.Parse()

	viper.SetConfigName("wrenderer")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.BindPFlags(pflag.CommandLine)

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file: %s\n", err)
	}

	if viper.GetBool("chromiumDebug") {
		viper.Set("debug", true)
	}

	var logger *slog.Logger
	if viper.GetBool("debug") {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	// Create boltdb connection
	db, err := bolt.Open(viper.GetString("cache.path"), 0600, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Error opening cache: %s", err))
		return err
	}
	defer db.Close()

	renderQueue := make(chan upAndRunWorker.RenderJob, viper.GetInt("queue.capacity"))
	semaphoreChan := make(chan struct{}, viper.GetInt("semaphore.capacity"))
	errChan := make(chan error, viper.GetInt("semaphore.capacity"))

	// Initialize a new instance of our application struct, containing the dependencies.
	app := &application{
		logger:           logger,
		port:             viper.GetInt("app.port"),
		db:               db,
		renderQueue:      renderQueue,
		sitemapSemaphore: semaphoreChan,
		errorChan:        errChan,
	}

	workerHandler := upAndRunWorker.Handler{
		Logger:      app.logger,
		DB:          db,
		RenderQueue: renderQueue,
		Semaphore:   semaphoreChan,
		ErrorChan:   errChan,
	}
	workerHandler.StartWorkers(viper.GetInt("queue.workers"))
	go workerHandler.StartCacheCleaner(viper.GetInt("cache.cleanupIntervalInMinutes"))

	logger.Info("starting server", slog.Int("port", app.port))
	err = http.ListenAndServe(fmt.Sprintf(":%d", app.port), app.routes())
	if err != nil {
		logger.Error(fmt.Sprintf("Error starting server: %s", err))
		return err
	}
	return nil
}
