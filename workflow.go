package hadrian

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	UnexpectedSigningErrorMsg string = "An unexpected error occurred during signing"
	UnexpectedParsingErrorMsg string = "An unexpected error occurred during parsing Response"
)

const GreetingTaskQueue = "GREETING_TASK_QUEUE"
const CryptoTaskQueue = "CRYPTO_TASK_QUEUE"
const CryptoSignTaskQueue = "CRYPTO_SIGN_QUEUE"

const defaultTimeout = time.Second * 2
const maxHookTimeout = time.Hour

type SignWorkflowRequest struct {
	Message    string
	Webhookurl string
	SignUrl    string
	ApiKey     string
	Timeout    time.Duration
}

type SignWorkflowResponse struct {
	Signature           []byte
	ProcessingAsWebhook bool
}

func SignWorkFlow(ctx workflow.Context, params SignWorkflowRequest) (SignWorkflowResponse, error) {
	timeout := defaultTimeout
	if params.Timeout != 0 {
		timeout = params.Timeout
	}
	var signResult UnreliableSignResponse
	signRequest := UnreliableSignRequest{
		Url:     params.SignUrl,
		Message: params.Message,
		ApiKey:  params.ApiKey,
	}

	// retry sign request to API
	unreliableSignRetryPolicy := &temporal.RetryPolicy{
		InitialInterval:        time.Second,
		BackoffCoefficient:     1.5,
		MaximumInterval:        60 * time.Second,
		MaximumAttempts:        0, // unlimited retries
		NonRetryableErrorTypes: []string{},
	}

	options := workflow.ActivityOptions{
		RetryPolicy:            unreliableSignRetryPolicy,
		ScheduleToCloseTimeout: timeout,
		TaskQueue:              CryptoSignTaskQueue,
	}
	activityCtx := workflow.WithActivityOptions(ctx, options)

	fut := workflow.ExecuteActivity(activityCtx, UnreliableSignActivity, signRequest)
	err := fut.Get(activityCtx, &signResult)
	result := SignWorkflowResponse{
		Signature:           signResult.EncodedMsg,
		ProcessingAsWebhook: false,
	}
	if timeout == defaultTimeout { // If activity failed due to not getting result in 3s -> Retry with long timeout and post result, else return result/err
		if temporal.IsTimeoutError(err) || temporal.IsApplicationError(err) {
			childWorkflowOptions := workflow.ChildWorkflowOptions{
				ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
			}
			childCtx := workflow.WithChildOptions(ctx, childWorkflowOptions)
			p := params
			p.Timeout = maxHookTimeout
			f := workflow.ExecuteChildWorkflow(childCtx, SignWorkFlow, p)
			g := f.GetChildWorkflowExecution() // Called only to ensure that it runs
			g.Get(ctx, nil)
			return SignWorkflowResponse{ProcessingAsWebhook: true}, nil
		}

		return result, err
	}

	// Now processing response as a webhook
	result.ProcessingAsWebhook = true

	if err != nil { // If LONG running activity exited with non-retriable errors (i.e. parsing/dialing errors), return the error
		return result, err
	}

	if params.Webhookurl == "" {
		return result, errors.New("no webhook url provided and remote signing took > 3 seconds")
	}

	postHookRetryPolicy := &temporal.RetryPolicy{
		InitialInterval: time.Second,
		MaximumAttempts: 1, // don't retry
	}
	optionsHook := workflow.ActivityOptions{
		RetryPolicy:         postHookRetryPolicy,
		StartToCloseTimeout: time.Minute,
		TaskQueue:           CryptoTaskQueue,
	}
	hookCtx := workflow.WithActivityOptions(ctx, optionsHook)
	var r string
	err = workflow.ExecuteActivity(hookCtx, SendPostHookActivity, SendPostHookParams{Url: params.Webhookurl, Body: signResult.EncodedMsg}).Get(activityCtx, &r)
	if err != nil {
		fmt.Println("error posting: ", err)
	}
	return result, err
}

type UnreliableSignRequest struct {
	Url     string
	Message string
	ApiKey  string
}

type UnreliableSignResponse struct {
	EncodedMsg []byte
}

func UnreliableSignActivity(ctx context.Context, params UnreliableSignRequest) (res UnreliableSignResponse, err error) {
	reqParams := url.Values{}
	reqParams.Add("message", params.Message)
	apiUrl := params.Url + "?" + reqParams.Encode()
	res = UnreliableSignResponse{}
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return res, err
	}
	req.Header.Add("Authorization", params.ApiKey)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil { // An unexpected error handling the request. 4xx/5xx status codes don't return errors
		return res, temporal.NewNonRetryableApplicationError(err.Error(), UnexpectedSigningErrorMsg, err)
	}

	// if client returned a bad status code, retry -> return a retriable error
	if response.StatusCode != 200 {
		return res, errors.New("Error from remote signing: " + strconv.Itoa(response.StatusCode))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return res, temporal.NewNonRetryableApplicationError(err.Error(), UnexpectedParsingErrorMsg, err)
	}
	err = response.Body.Close()
	if err != nil {
		return res, temporal.NewNonRetryableApplicationError(err.Error(), UnexpectedParsingErrorMsg, err)

	}
	res.EncodedMsg = body
	return res, nil
}

type VerifyWorkflowRequest struct {
	Message    string
	Signature  string
	Webhookurl string
	VerifyUrl  string
	ApiKey     string
	Timeout    time.Duration
}

type VerifyWorkflowResponse struct {
	Message             []byte
	ProcessingAsWebhook bool
}

type UnreliableVerifyRequest struct {
	Url       string
	Message   string
	Signature string
	ApiKey    string
}

type UnreliableVerifyResponse struct {
	Body []byte
}

func VerifyWorkflow(ctx workflow.Context, params VerifyWorkflowRequest) (VerifyWorkflowResponse, error) {
	timeout := defaultTimeout
	if params.Timeout != 0 {
		timeout = params.Timeout
	}
	var verifyResult UnreliableVerifyResponse
	verifyRequest := UnreliableVerifyRequest{
		Url:       params.VerifyUrl,
		Message:   params.Message,
		Signature: params.Signature,
		ApiKey:    params.ApiKey,
	}

	// retry sign request to API
	unreliableVerifyRetryPolicy := &temporal.RetryPolicy{
		InitialInterval:        time.Second,
		BackoffCoefficient:     1.5,
		MaximumInterval:        60 * time.Second,
		MaximumAttempts:        0, // unlimited retries
		NonRetryableErrorTypes: []string{},
	}

	options := workflow.ActivityOptions{
		RetryPolicy:            unreliableVerifyRetryPolicy,
		ScheduleToCloseTimeout: timeout,
		TaskQueue:              CryptoSignTaskQueue,
	}
	activityCtx := workflow.WithActivityOptions(ctx, options)

	fut := workflow.ExecuteActivity(activityCtx, UnreliableVerifyActivity, verifyRequest)
	err := fut.Get(activityCtx, &verifyResult)
	result := VerifyWorkflowResponse{
		Message:             []byte("Signature verified as valid!"),
		ProcessingAsWebhook: false,
	}
	if timeout == defaultTimeout { // If activity failed due to not getting result in 3s -> Retry with long timeout and post result, else return result/err
		if temporal.IsTimeoutError(err) || temporal.IsApplicationError(err) {
			childWorkflowOptions := workflow.ChildWorkflowOptions{
				ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
			}
			childCtx := workflow.WithChildOptions(ctx, childWorkflowOptions)
			p := params
			p.Timeout = maxHookTimeout
			f := workflow.ExecuteChildWorkflow(childCtx, VerifyWorkflow, p)
			g := f.GetChildWorkflowExecution() // Called only to ensure that it runs
			g.Get(ctx, nil)
			return VerifyWorkflowResponse{ProcessingAsWebhook: true}, nil
		}

		return result, err
	}

	// Now processing response as a webhook
	result.ProcessingAsWebhook = true

	if err != nil { // If LONG running activity exited with non-retriable errors (i.e. parsing/dialing errors), return the error
		return result, err
	}

	if params.Webhookurl == "" {
		return result, errors.New("no webhook url provided and remote signing took > 3 seconds")
	}

	postHookRetryPolicy := &temporal.RetryPolicy{
		InitialInterval: time.Second,
		MaximumAttempts: 1, // don't retry
	}
	optionsHook := workflow.ActivityOptions{
		RetryPolicy:         postHookRetryPolicy,
		StartToCloseTimeout: time.Minute,
		TaskQueue:           CryptoTaskQueue,
	}
	hookCtx := workflow.WithActivityOptions(ctx, optionsHook)
	var r string
	err = workflow.ExecuteActivity(hookCtx, SendPostHookActivity, SendPostHookParams{Url: params.Webhookurl, Body: verifyResult.Body}).Get(activityCtx, &r)
	if err != nil {
		fmt.Println("error posting: ", err)
	}
	return result, err

}

func UnreliableVerifyActivity(ctx context.Context, params UnreliableVerifyRequest) (res UnreliableVerifyResponse, err error) {
	reqParams := url.Values{}
	reqParams.Add("message", params.Message)
	reqParams.Add("signature", params.Signature)
	apiUrl := params.Url + "?" + reqParams.Encode()
	res = UnreliableVerifyResponse{}
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return res, err
	}
	req.Header.Add("Authorization", params.ApiKey)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil { // An unexpected error handling the request. 4xx/5xx status codes don't return errors
		return res, temporal.NewNonRetryableApplicationError(err.Error(), UnexpectedSigningErrorMsg, err)
	}

	// if client returned a bad status code, retry -> return a retriable error
	if response.StatusCode != 200 {
		body, _ := io.ReadAll(response.Body)
		return res, errors.New("Error from remote signing: " + strconv.Itoa(response.StatusCode) + string(body))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return res, temporal.NewNonRetryableApplicationError(err.Error(), UnexpectedParsingErrorMsg, err)
	}
	err = response.Body.Close()
	if err != nil {
		return res, temporal.NewNonRetryableApplicationError(err.Error(), UnexpectedParsingErrorMsg, err)

	}
	res.Body = body
	return res, nil
}

type SendPostHookParams struct {
	Body []byte
	Url  string
}

func SendPostHookActivity(ctx context.Context, params SendPostHookParams) (string, error) {
	client := &http.Client{}
	fmt.Println("POSTING TO: ", params.Url, string(params.Body))
	req, err := http.NewRequest("POST", params.Url, bytes.NewBuffer(params.Body))
	if err != nil {
		fmt.Println(err, err.Error())
		return "", err
	}
	_, err = client.Do(req)
	return "", err
}
