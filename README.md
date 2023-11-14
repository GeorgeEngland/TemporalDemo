# Hadrian('s wall) - the API
The Wall: Acted as a boundary between the seemingly civilized Roman Empire and the danger of the Highlands of Scotland.

The API: Hadrian the API intends to protect the user of our API from the un-reliable and treacherous 3rd party API by exposing a robust and reliable one.

## Quickstart

Set APIKEY in dockercompose

L16 APIKEY=6a....

```
docker compose up -d --build
```
Wait 1 minute for all services to be operational

Send Curl

```
curl -v -X GET "127.0.0.1:9999/sign?message=helloworld" -d '{"webhookurl":"https://httpbin.org/post"}'

curl -v -X GET "127.0.0.1:9999/verify?message=helloworld&signature=C5PMEP8AQVFhvppZflU3pEbmpEyRR4VRihZR1sGkFjWz_mQv9lkDnp8Gdffi_IFDX9HnYfrkX6ms-HCAAcaftpNghbwvCEMzM1f9284tsgm0axzHnIpnEEhH_FFg1-LDB_sVNxAZ2UTCUq7bN5GcCB1o7dOz70WYa4AFz39PHNmrHjJXrXXJYDXmJUJs1NlfFJnFRGMEmeRegGuXa9sfPQhsJNetuhx_5O4sXHcEqDkhOYozTXb2806m_545Mh6Nlm5S-KTkYS96ATPFEmhrb6ELKN5AUeLCR5Hjreg54fo1nTSGpvDA6Xs-U8AA0o2XUONw0ov4XFEgXQwENnw4rQ==" -d '{"webhookurl":"https://httpbin.org/post"}'
```

View Workflow Status UI: 127.0.0.1:8080

## Endpoint Docs

If webhookurl is set in the body, a post request will be sent with the result to that url

### GET /sign

Params: message (required)

Body (not required): {"webhookurl": "YOUR_URL"}

### GET /verify

Params: message (required), signature (required)

Body (not required): {"webhookurl": "YOUR_URL"}

## Overview

Here is implemented a dynamically horizontally scalable, resilient, solution.

i.e. We can scale our Http Server horizontally idependently of our workers. 

e.g. if the traffic stays the same, but the resources required to run the task goes up significantly, we can increase our workers.

e.g. If traffic increases to beyond the limit of our http server, but the tasks are still lightweight, we can scale the http server separately.

e.g. If we want to give customers much faster responses, deploy the http servers in different regions, close to the customers cheaply, and keep our big worker servers near our databases

We are resilient to outages of the 3rd party API, outages of our Worker Servers, and outages of our HTTP server.

When our temporal (database) server goes down, our API will respond with Internal Server Error since we cannot ensure determinism in this situation.

The two endpoints operate very similarly by using Temporal as the backend for our deterministic and idempotent work queue.

### Core Components:

- Workflow Server (Processes top level workflows and unconstrained activities)
- Activity Server (Processes Rate Limited Activities to ensure we don't exceed rate limits of 3rd Party API)
- Http Server (127.0.0.1:9999)
- Temporal Server (127.0.0.1:7233)

### Peripheral Components

- Workflow Admin UI (127.0.0.1:8080)
```
Request
|
| /sign or /verify
|
Backend HTTP Server
|
|-- Initiates a Sign/Verify Workflow By Registering the workflow in the database along with its inputs
    | -- Attempts to complete workflow in 2s
    | -- If timeout exceeded, Start a long running workflow to call the Webhook URL
|-- If initial workflow succeeds, return response to user, otherwise, on succesful return of workflow, return 202 to user
```
### Workflows
'A Workflow Execution effectively executes once to completion'

By registering a workflow along with all its inputs, ensuring idempotency and determinism, workflows can re-start/continue across server restarts

### Activities
A non-deterministic piece of code (e.g. an API call).

We put our request to the 3rd party API in activities and define explicit Retry Policies.

We have a separate Worker for the API with a rate limit, not allowing more than 10 Activity Scehdulling Events per minute.

