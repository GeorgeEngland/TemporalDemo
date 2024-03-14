# Determinism Into
We build an example temporal workflow system, allowing scaling of separate web/workflow/acitivity servers

## Quickstart

```
docker compose up -d --build
```
Wait 1 minute for all services to be operational

Send Curl

```
curl -v -X GET "127.0.0.1:9999/"

curl -v -X GET "127.0.0.1:9999/start" -d '{"SucceedOneIn": 30, "Message": "Hello World"}'
```

View Workflow Status UI: 127.0.0.1:8080

## Endpoint Docs

### GET /

Params: None

Body (not required): None

### GET /start

Params: None

Body (not required): {"SucceedOneIn": 3, "Message": "hello"}

SucceedOneIn = Chance of the unreliable activity succeeding

Message = Message to display on success

## Overview

Here is implemented a dynamically horizontally scalable, resilient, workflow service.

i.e. We can scale our Http Server horizontally idependently of our workers. 

e.g. if the traffic stays the same, but the resources required to run the task goes up significantly, we can increase our workers.

e.g. If traffic increases to beyond the limit of our http server, but the tasks are still lightweight, we can scale the http server separately.

e.g. If we want to give customers much faster responses, deploy the http servers in different regions, close to the customers cheaply, and keep our big worker servers near our databases, we can do so.

We are resilient to outages of the unreliable activity, outages of our Worker Servers, outages of our workflow servers and outages of our HTTP server.

When our temporal (database) server goes down, our API will respond with Internal Server Error since we cannot ensure determinism in this situation.

### Core Components:

- Workflow Server (Processes top level workflows)
- Activity Server (Processes Rate Limited/Unreliable & non deterministic Activities)
- Http Server (127.0.0.1:9999)
- Temporal Server (127.0.0.1:7233)

### Peripheral Components

- Workflow Admin UI (127.0.0.1:8080)
```
Request
|
| /start
|
Backend HTTP Server
|
|-- Initiates a Start Workflow By Registering the workflow in the database along with its inputs
|
|-- Return to the client with success response
```
### Workflows
'A Workflow Execution effectively executes once to completion'

By registering a workflow along with all its inputs, ensuring idempotency and determinism, workflows can re-start/continue across server restarts

### Activities
A non-deterministic piece of code (e.g. an API call).
