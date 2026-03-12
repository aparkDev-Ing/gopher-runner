package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	//"runtime"

	//"github.com/joho/godotenv"

	constants "gitlab.com/aparkdev-ing/gopher-runner/constants"
)

func jobHandler(ctx context.Context) {

	jobQueue := make(chan *JobResponse, 10)

	var countDownLatch sync.WaitGroup

	noOfThread := 3 //runtime.NumCpu()
	for i := 0; i < noOfThread; i++ {
		countDownLatch.Add(1)
		func() {
			//defer countDownLatch.Done()
			go worker(ctx, jobQueue, i+1, &countDownLatch)
		}()
	}

	for {

		select {
		case <-ctx.Done():
			{
				fmt.Println("\n[Main] Shutdown signal received. Stopping job requests...")
				close(jobQueue)
				countDownLatch.Wait()
				return
			}

		default:
			{
				job, err := requestJob(ctx)

				if err != nil {
					fmt.Println(err)
					time.Sleep(10 * time.Second)
					continue
				}

				if job != nil {
					jobQueue <- job
				} else {
					fmt.Printf("No Job Found. ")
				}

				time.Sleep(1 * time.Second)
			}
		}
	}
}

func worker(ctx context.Context, jobQueue <-chan *JobResponse, threadId int, countDownLatch *sync.WaitGroup) {

	defer countDownLatch.Done()
	for job := range jobQueue {
		printLog(job.ID, threadId, constants.CLAIMED)
		err := processJob(ctx, job)
		if err != nil {
			fmt.Println(err)
		}
		printLog(job.ID, threadId, constants.FINISHED)
	}
}

func printLog(jobId int, threadId int, jobType string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] | Thread ID: %d| Successfully %v Job ID: %d\n", timestamp, threadId, jobType, jobId)
}

func processJob(ctx context.Context, jobResponse *JobResponse) (err error) {

	token := jobResponse.Token
	heartBeat := make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		//heartbeatMsg := "[Heartbeat] | " + strconv.Itoa(jobResponse.ID) + " " + constants.HEARTBEAT_GENERIC_MESSAGE

		for {
			select {
			case <-ticker.C:
				_, err := updateJobStatus(ctx, jobResponse.ID, constants.RUNNING, "", token)
				if err != nil {
					fmt.Printf("[Heartbeat] | [JobId %d] Heartbeat failed: %v\n", jobResponse.ID, err)
				}
				fmt.Printf("[Heartbeat] | Heartbeat sent for Job %d\n", jobResponse.ID)
			case <-heartBeat:
				fmt.Printf("[Heartbeat] |Stopping heartbeat for Job %d\n", jobResponse.ID)
				return
			}
		}

	}()

	currentEnv := os.Environ()

	for _, variable := range jobResponse.Variables {
		kvPair := fmt.Sprintf("%s=%s", variable.Key, variable.Value)
		currentEnv = append(currentEnv, kvPair)
	}

	var log strings.Builder
	lastOffset := 0

	fmt.Printf("--- Job Id: %d\n", jobResponse.ID)

	for i, step := range jobResponse.Steps {
		fmt.Printf("--- Step%d: %v\n", i+1, step.Name)

		stepLog := fmt.Sprintf("\nStep%d: %v \n", i+1, step.Name)
		log.WriteString(stepLog)

		for _, v := range step.Script {
			fmt.Printf("--- Execute Command: %v\n", v)

			shellCmd := exec.Command("bash", "-c", v)
			shellCmd.Env = currentEnv
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			// Corrected line:
			header := fmt.Sprintf("\n\033[32;1m[%s] $ %s\033[0m\n", timestamp, v)
			log.WriteString(header)

			output, err := shellCmd.CombinedOutput()
			log.WriteString(string(output))

			fullLog := log.String()
			currentLog := fullLog[lastOffset:]

			if len(currentLog) > 0 {
				traceErr := updateJobTrace(jobResponse.ID, currentLog, lastOffset, token)

				if traceErr != nil {
					fmt.Printf("[Job %d] Warning: Trace update failed: %v\n", jobResponse.ID, traceErr)
				}
				lastOffset += len(currentLog)
			}

			if err != nil {
				heartBeat <- true
				fmt.Printf("Script Failure Output: %s\n", string(output))
				footer := fmt.Sprintf("\n\033[31;1mERROR: Command failed: %v\033[0m\n", err)
				log.WriteString(footer)
				_, updateStatusError := updateJobStatus(ctx, jobResponse.ID, constants.FAILED, log.String(), token)
				if updateStatusError != nil {
					return errorLogger(updateStatusError, "Error Occurred While Updating Job Status")
				}
				return errorLogger(err, "Script Failed With Error")
			}

			fmt.Printf("--- Command Output: %s\n", string(output))
		}
	}

	//to tell heartbeat thread to stop
	heartBeat <- true

	_, updateStatusError := updateJobStatus(ctx, jobResponse.ID, constants.SUCCESS, log.String(), token)

	if updateStatusError != nil {
		return errorLogger(updateStatusError, "Error Occurred While Updating Job Status")
	}

	return nil
}
