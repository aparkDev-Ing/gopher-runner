Gopher-Runner: Building a Custom GitLab Runner
I’ve always been curious about what happens behind the scenes when a CI/CD job is triggered. To find out, I decided to build my own GitLab Runner using Go. This project isn't just about making something that works; it's a deep dive into GitLab's internal APIs, asynchronous job handling, and the overall CI/CD lifecycle.

Why I built this :
The goal was simple: Deconstruct the magic. I wanted to see how the GitLab Server manages runners and how those runners, in turn, handle jobs at the system level.By building Gopher-Runner, I was able to:

1) Reverse-Engineer the GitLab Core APIs

I mapped out the essential communication flow required for a functional runner:

Verification: POST /api/v4/runners/verify — Ensuring the runner is healthy and authorized.

Job Request: POST /api/v4/jobs/request — Polling the server for available work.

Status Updates: PUT /api/v4/jobs/:id — Reporting state changes (Running, Success, Failed).

Log Streaming: PATCH /api/v4/jobs/:id/trace — Sending script output (I/O logs) back to the server in real-time.

2) I designed the runner to handle high-volume job polling and maintenance without blocking the system:

Main Thread (Producer): Constantly requests new jobs from the GitLab server and pushes them into a central Channel.

Worker Pool (Consumers): Multiple Goroutines poll from the channel. Once a job is claimed, a worker executes the script and handles all status/log updates independently.

Asynchronous Heartbeat: Implemented a background process that utilizes a Go Ticker and Channel to send a "Heartbeat" to the GitLab server every 30 seconds. This ensures the server knows the runner is healthy and prevents active jobs from being timed out or marked as "stuck."

Graceful Fault Handling: Each worker is designed to catch failures during execution, ensuring the GitLab server is updated with a failed status rather than leaving the job hanging.

Graceful Shutdown: Implemented signal handling so that even if os.Exit is triggered, the runner waits for active Goroutines to finish their current tasks before shutting down, preventing corrupted job states.


3) I gained a granular understanding of how a high-level .gitlab-ci.yml definition is broken down into individual bash commands and how those steps are orchestrated by the runner.

Language: Go (The perfect tool for high-performance, concurrent systems)

🏁 Getting Started
Prerequisites
Go 1.21+

A GitLab instance and a Runner registration token.

Quick Start

Bash
git clone https://gitlab.com/aparkdev-ing/gopher-runner.git
cd gopher-runner
go build -o gopher-runner
export GITLAB_TOKEN="Token"

Author: Aaron Park

Status: Under active development. Feel free to reach out if you have questions!