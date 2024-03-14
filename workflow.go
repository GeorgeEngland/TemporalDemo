package hadrian

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	UnexpectedParsingErrorMsg string = "An unexpected error occurred during parsing Response"
)

const WorkflowTaskQueue = "WORKFLOW_QUEUE"
const UnreliableActivityTaskQueue = "ACTIVITYQUEUE"

const defaultTimeout = time.Second * 2

type WorkflowRequest struct {
	Message      string
	Timeout      time.Duration
	SucceedOneIn int
}

type DeterministicWorkflowResponse struct {
	WorkflowSuccessMessage string
}

func DeterministicWorkflow(ctx workflow.Context, params WorkflowRequest) (DeterministicWorkflowResponse, error) {
	timeout := defaultTimeout
	if params.Timeout != 0 {
		timeout = params.Timeout
	}
	// retry sign request to API
	// Limits Per api request basis
	unreliableActivityRetryPolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Millisecond * 100,
		BackoffCoefficient: 1,
		MaximumInterval:    time.Millisecond * 200,
		MaximumAttempts:    30,
	}

	options := workflow.ActivityOptions{
		RetryPolicy:            unreliableActivityRetryPolicy,
		ScheduleToCloseTimeout: timeout,
		TaskQueue:              UnreliableActivityTaskQueue,
	}
	activityCtx := workflow.WithActivityOptions(ctx, options)
	var signResult UnreliableActivityResponse
	input := UnreliableActivityInput{
		Message:      params.Message,
		SucceedOneIn: params.SucceedOneIn,
	}
	fut := workflow.ExecuteActivity(activityCtx, UnreliableActivity, input)
	err := fut.Get(activityCtx, &signResult)
	result := DeterministicWorkflowResponse{
		WorkflowSuccessMessage: signResult.SuccessMessage,
	}

	return result, err
}

type UnreliableActivityResponse struct {
	SuccessMessage string
}
type UnreliableActivityInput struct {
	Message      string
	SucceedOneIn int
}

func UnreliableActivity(ctx context.Context, input UnreliableActivityInput) (res UnreliableActivityResponse, err error) {
	fmt.Println("Attempting Unreliable activity")

	if rand.Intn(input.SucceedOneIn) <= 1 {
		return UnreliableActivityResponse{
			SuccessMessage: input.Message + " Success!",
		}, nil
	}
	fmt.Println("Received Unexpected Error")
	// return res, temporal.NewNonRetryableApplicationError(err.Error(), UnexpectedParsingErrorMsg, err)
	return res, errors.New("Unexpected Error!")
}
