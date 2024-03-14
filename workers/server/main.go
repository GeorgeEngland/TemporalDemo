package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/GeorgeEngland/hadrian"
	"go.temporal.io/sdk/client"
)

func main() {
	c, err := client.Dial(client.Options{HostPort: "temporal:7233"})
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	defer c.Close()

	m := http.NewServeMux()
	m.HandleFunc("/start", WithTemporal(&c, signHandler))
	m.HandleFunc("/", helloHandle)
	http.ListenAndServe(":9999", m)
}


func helloHandle(w http.ResponseWriter, r *http.Request){
	w.Write([]byte("hello"))
}

func signHandler(ctx context.Context, c *client.Client, w http.ResponseWriter, r *http.Request) {
	signParams, err := parseSignRequest(w, r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	fmt.Println("Starting Workflow with params: ", signParams)

	workflowRes, err := (*c).ExecuteWorkflow(ctx, client.StartWorkflowOptions{TaskQueue: hadrian.WorkflowTaskQueue}, hadrian.DeterministicWorkflow,
		*signParams)
	if err != nil {
		fmt.Println(err, err.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("internal server error"))
		return
	}

	var workflowResData hadrian.DeterministicWorkflowResponse
	err = workflowRes.Get(ctx, &workflowResData)

	if err != nil {
		fmt.Println(err, err.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("internal server error"))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(workflowResData.WorkflowSuccessMessage))

}

type HandlerFunc func(ctx context.Context, tmprl *client.Client, w http.ResponseWriter, r *http.Request)

func WithTemporal(tmprl *client.Client, handler HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(r.Context(), tmprl, w, r)
	}
}


type Payload struct {
	SucceedOneIn int `json:"SucceedOneIn"`
	Message string `json:"Message"`
}

func parseSignRequest(w http.ResponseWriter, r *http.Request) (*hadrian.WorkflowRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return nil, err
	}
	var payload Payload
	if len(body) > 0 {
		err = json.Unmarshal(body, &payload)
		if err != nil {
			w.WriteHeader(400)
			http.Error(w, "Error decoding JSON body", http.StatusBadRequest)
			return nil, err
		}
	}

	signParams := &hadrian.WorkflowRequest{
		SucceedOneIn: payload.SucceedOneIn,
		Message:    payload.Message,
		Timeout: time.Minute * 2,
	}
	if signParams.SucceedOneIn <= 0 {
		signParams.SucceedOneIn=1
	}
	return signParams, nil
}
