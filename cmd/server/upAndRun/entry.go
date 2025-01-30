package upAndRun

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type application struct {
	logger *slog.Logger
	port   int
}

func Start() {
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

	// Initialize a new instance of our application struct, containing the dependencies.
	app := &application{
		logger: logger,
		port:   viper.GetInt("app.port"),
	}

	logger.Info("starting server", slog.Int("port", app.port))
	err = http.ListenAndServe(fmt.Sprintf(":%d", app.port), app.routes())
}
