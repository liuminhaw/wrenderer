package upAndRun

import (
	"crypto/tls"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/wrenderer/cmd/shared/localEnv"
	"github.com/liuminhaw/wrenderer/cmd/worker/upAndRunWorker"
	"github.com/spf13/pflag"
)

func Start() error {
	pflag.String("app.addr", ":8080", "Server address")
	pflag.Bool("debug", false, "Enable debug mode")
	pflag.Bool(
		"chromiumDebug",
		false,
		"Enable chromium debug mode, enable this will automatically enable debug mode",
	)
	pflag.Parse()

	// Initialize a viper instance
	vConfig := localEnv.InitConfig()
	if err := localEnv.ConfigSetup(vConfig); err != nil {
		log.Fatalf("Error reading config file: %s\n", err)
	}

	if vConfig.GetBool("chromiumDebug") {
		vConfig.Set("debug", true)
	}

	var logger *slog.Logger
	if vConfig.GetBool("debug") {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	// Create boltdb connection
	db, err := bolt.Open(vConfig.GetString("cache.path"), 0600, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Error opening cache: %s", err))
		return err
	}
	defer db.Close()

	renderQueue := make(chan upAndRunWorker.RenderJob, vConfig.GetInt("queue.capacity"))
	semaphoreChan := make(chan struct{}, vConfig.GetInt("semaphore.capacity"))
	errChan := make(chan error, vConfig.GetInt("semaphore.capacity"))

	// Initialize a new instance of our application struct, containing the dependencies.
	app := &application{
		logger:           logger,
		addr:             vConfig.GetString("app.addr"),
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
	workerHandler.StartWorkers(vConfig)
	go workerHandler.StartCacheCleaner(vConfig.GetInt("cache.cleanupIntervalInMinutes"))

	srv := &http.Server{
		Addr:     app.addr,
		Handler:  app.routes(vConfig),
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info(
		"starting server",
		slog.String("address", app.addr),
		slog.Bool("tls", vConfig.GetBool("app.tls")),
	)
	if vConfig.GetBool("app.tls") {
		srv.TLSConfig = &tls.Config{
			CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		}
		err = srv.ListenAndServeTLS(
			vConfig.GetString("app.tlsCert"),
			vConfig.GetString("app.tlsKey"),
		)
	} else {
		err = srv.ListenAndServe()
	}

	if err != nil {
		logger.Error(fmt.Sprintf("Error starting server: %s", err))
		return err
	}
	return nil
}
