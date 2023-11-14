package main

import (
	"log"

	"github.com/GeorgeEngland/hadrian"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create the client object just once per process
	c, err := client.Dial(client.Options{HostPort: "temporal:7233"})
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	defer c.Close()

	// This worker hosts both Workflow and Activity functions
	w := worker.New(c, hadrian.CryptoTaskQueue, worker.Options{})
	w.RegisterWorkflow(hadrian.SignWorkFlow)
	w.RegisterWorkflow(hadrian.VerifyWorkflow)
	w.RegisterActivity(hadrian.SendPostHookActivity)
	// Start listening to the Task Queue
	err = w.Run(worker.InterruptCh())

	if err != nil {
		log.Fatalln("unable to start Worker", err)
	}
}
