package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/GeorgeEngland/hadrian"
	"go.temporal.io/sdk/client"
)

const (
	MISSING_MESSAGE_RESPONSE   = "Must specify a message."
	MISSING_SIGNATURE_RESPONSE = "Must specify a signature."
	INVALID_SIGNATURE_RESPONSE = "Invalid signature for message!"

	SIGNING_URL   = "https://hiring.api.synthesia.io/crypto/sign"
	VERIFYING_URL = "https://hiring.api.synthesia.io/crypto/verify"
)

var apiKey string

func main() {
	// both endpoints accept webhookurl param

	apiKey = os.Getenv("APIKEY")
	if apiKey == "" {
		log.Fatal("No ENV Var APIKEY")
	}

	c, err := client.Dial(client.Options{HostPort: "temporal:7233"})
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	defer c.Close()

	// /sign (no params) -> 400: Must specify a message.
	// /sign?message=hello world -> 200 MvM5Q0wWkOQAYtZsdQboLnp0au-vLguBquryFlBWmicV62p4xNZQSxAg7E7wjfFLx9gAxrUWKlRTG484yTQNdWlD691IUGwGsfm-I94qJ7LRztajSetWReAnnTHTvq7WbPlm1j6jStKI_oSDsyLL4id9m7zSDooDEqO8bkXnD2Fe8iCB1JKhJ3NyEDUYvrTWp13DxAIpeN6CUgYBbzg9rdLSTFYciTDoHDZtpyrvt54dHqgYVXD_FRbzm1-mDkNLW15HbqYFcwHmx6gCQEPI8wnfQh5wPyHov5UHCHXtTl_jyZFkRA3N2Su93wxcWLXy9WGN9uOyw1L2HMgwpolcMg==
	// /verify (no params) -> 400: Must specify a message.
	// /verify?message (no params) -> 400: Must specify a signature.
	// /verify?message&signature -> 400: Invalid signature for message!
	// /verify?message=hello world&signature=MvM... -> 200 Signature verified as valid!
	m := http.NewServeMux()
	m.HandleFunc("/sign", WithTemporal(&c, signHandler))
	m.HandleFunc("/verify", WithTemporal(&c, verifyHandler))
	http.ListenAndServe(":9999", m)
}

type WebhookPayload struct {
	WebhookUrl string `json:"webhookurl"`
}

func signHandler(ctx context.Context, c *client.Client, w http.ResponseWriter, r *http.Request) {
	signParams, err := parseSignRequest(w, r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	workflowRes, err := (*c).ExecuteWorkflow(ctx, client.StartWorkflowOptions{TaskQueue: hadrian.CryptoTaskQueue}, hadrian.SignWorkFlow,
		*signParams)
	if err != nil {
		fmt.Println(err, err.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("internal server error"))
		return
	}

	var workflowResData hadrian.SignWorkflowResponse
	err = workflowRes.Get(ctx, &workflowResData)

	if err != nil {
		fmt.Println(err, err.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("internal server error"))
		return
	}
	if workflowResData.ProcessingAsWebhook {
		w.WriteHeader(http.StatusAccepted)
		fmt.Println("Scheduled task")
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(workflowResData.Signature)

}

func verifyHandler(ctx context.Context, c *client.Client, w http.ResponseWriter, r *http.Request) {
	verifyParams, err := parseVerifyRequest(w, r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	workflowRes, err := (*c).ExecuteWorkflow(ctx, client.StartWorkflowOptions{TaskQueue: hadrian.CryptoTaskQueue}, hadrian.VerifyWorkflow,
		*verifyParams)
	if err != nil {
		fmt.Println(err, err.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("internal server error"))
		return
	}

	var workflowResData hadrian.VerifyWorkflowResponse
	err = workflowRes.Get(ctx, &workflowResData)

	if err != nil {
		fmt.Println(err, err.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("internal server error"))
		return
	}
	if workflowResData.ProcessingAsWebhook {
		w.WriteHeader(http.StatusAccepted)
		fmt.Println("Scheduled task")
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(workflowResData.Message)
}

type HandlerFunc func(ctx context.Context, tmprl *client.Client, w http.ResponseWriter, r *http.Request)

func WithTemporal(tmprl *client.Client, handler HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(r.Context(), tmprl, w, r)
	}
}

func parseSignRequest(w http.ResponseWriter, r *http.Request) (*hadrian.SignWorkflowRequest, error) {
	params := r.URL.Query()

	has := params.Has("message")
	if !has {
		w.WriteHeader(400)
		w.Write([]byte(MISSING_MESSAGE_RESPONSE))
		return nil, errors.New("missing message")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return nil, err
	}
	var payload WebhookPayload
	if len(body) > 0 {
		err = json.Unmarshal(body, &payload)
		if err != nil {
			w.WriteHeader(400)
			http.Error(w, "Error decoding JSON body", http.StatusBadRequest)
			return nil, err
		}
	}

	signParams := &hadrian.SignWorkflowRequest{
		ApiKey:     apiKey,
		SignUrl:    SIGNING_URL,
		Webhookurl: payload.WebhookUrl,
		Message:    params.Get("message"),
	}
	return signParams, nil
}

func parseVerifyRequest(w http.ResponseWriter, r *http.Request) (*hadrian.VerifyWorkflowRequest, error) {
	params := r.URL.Query()

	has := params.Has("message")
	if !has {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(MISSING_MESSAGE_RESPONSE))
		return nil, errors.New("missing message")
	}

	has = params.Has("signature")
	if !has {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(MISSING_SIGNATURE_RESPONSE))
		return nil, errors.New("missing signature")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return nil, err
	}
	var payload WebhookPayload
	if len(body) > 0 {
		err = json.Unmarshal(body, &payload)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			http.Error(w, "Error decoding JSON body", http.StatusBadRequest)
			return nil, err
		}
	}

	signParams := &hadrian.VerifyWorkflowRequest{
		ApiKey:     apiKey,
		VerifyUrl:  VERIFYING_URL,
		Webhookurl: payload.WebhookUrl,
		Message:    params.Get("message"),
		Signature:  params.Get("signature"),
	}
	return signParams, nil
}
