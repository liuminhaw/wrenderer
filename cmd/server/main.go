package main

import (
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/liuminhaw/wrenderer/cmd/server/awsLambda"
	"github.com/liuminhaw/wrenderer/cmd/server/upAndRun"
)

func main() {
	if _, exists := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); exists {
		runInLambda()
	} else {
		run()
	}
}

func run() {
	upAndRun.Start()
}

func runInLambda() {
	lambda.Start(awsLambda.LambdaHandler)
}
