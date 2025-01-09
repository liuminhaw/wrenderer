package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/liuminhaw/wrenderer/cmd/server/awsLambda"
	"github.com/spf13/viper"
)

func main() {
	if _, exists := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); exists {
		runInLambda()
	} else {
		run()
	}
}

func run() {
	viper.SetConfigName("wrenderer")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file: %s\n", err)
	}

	appPort := viper.GetInt("app.port")
	log.Printf("config app port: %d", appPort)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /render", pageRenderWithConfig(viper.GetViper()))

	log.Printf("server listening on %d port", appPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", appPort), mux))
}

func runInLambda() {
	fmt.Println("Running in AWS Lambda custom image")
	lambda.Start(awsLambda.LambdaHandler)
}
