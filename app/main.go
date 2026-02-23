package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	//"runtime"

	"github.com/joho/godotenv"

	constants "gitlab.com/aparkdev-ing/gopher-runner/constants"
)

var AppConfig Config

var httpClient *http.Client = &http.Client{
	Timeout: 20 * time.Second,
}

func healthCheck() {

	fmt.Println("Verification Starts")

	resp, err := verifyRunner()

	if err != nil {
		// panic(err)
		fmt.Println(err)
		os.Exit(1)
	}

	if resp != http.StatusOK {
		fmt.Printf("[%s] Runner token rejected by GitLab (Status: %d)\n",
			time.Now().Format(time.RFC3339), resp)
		os.Exit(1)
	}

	fmt.Println("Verification Completed Successfully | Status Code:", resp)

}

func formHttpRequest(url string, token string, requestStruct any, httpMethod string) (*http.Request, error) {

	//if performacne is needed
	//json.NewEncoder(someWriter).Encode(myStruct).

	jsonData, err := json.Marshal(requestStruct)
	if err != nil {
		return nil, errorLogger(err, constants.SERIALIZATION_ERROR)
	}

	req, err := http.NewRequest(httpMethod, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errorLogger(err, constants.HTTP_REQUEST_FORM_ERROR)
	}

	req.Header.Set(constants.CONTENT_TYPE, constants.JSON)

	return req, nil
}

// func createHttpClient(httpRequest *http.Request) *http.Client {

// 	return &http.Client{
// 		Timeout: 20 * time.Second,
// 	}
// }

// think of using validateResponse helper here too
func verifyRunner() (int, error) {

	token := AppConfig.RegistrationToken
	url := AppConfig.VerifyURL

	verifyRunnerRequest := VerifyRequest{
		Token: token,
	}

	req, err := formHttpRequest(url, token, verifyRunnerRequest, constants.POST_METHOD)

	if err != nil {
		return -1, err
	}

	// client := createHttpClient(req)

	resp, err := httpClient.Do(req)

	if err != nil {
		return -1, errorLogger(err, constants.HTTP_REQUEST_ERROR)
	}

	statusCode := resp.StatusCode

	defer resp.Body.Close()
	return statusCode, nil
}

func requestJob() (*JobResponse, error) {

	token := AppConfig.RegistrationToken
	url := AppConfig.RequestURL

	requestJobStruct := JobRequest{
		Token:   token,
		TagList: []string{constants.GOPHER_RUNNER},
	}

	req, err := formHttpRequest(url, token, requestJobStruct, constants.POST_METHOD)

	if err != nil {
		return nil, err
	}

	//client := createHttpClient(req)

	resp, err := httpClient.Do(req)

	if err != nil {
		return nil, errorLogger(err, constants.HTTP_REQUEST_ERROR)
	}

	defer resp.Body.Close()

	decode, err := validateResponse(resp)

	if !decode {
		return nil, err
	}

	var jobResponse JobResponse

	deserializationError := json.NewDecoder(resp.Body).Decode(&jobResponse)

	//jsonData, _ := json.Marshal(jobResponse)
	//fmt.Println("Job Response:", string(jsonData))

	if deserializationError != nil {
		return nil, errorLogger(deserializationError, constants.DESERIALIZATION_ERROR)
	}

	return &jobResponse, nil
}

func processJob(jobResponse *JobResponse) (err error) {

	token := jobResponse.Token

	heartBeat := make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		//heartbeatMsg := "[Heartbeat] | " + strconv.Itoa(jobResponse.ID) + " " + constants.HEARTBEAT_GENERIC_MESSAGE

		for {
			select {
			case <-ticker.C:
				_, err := updateJobStatus(jobResponse.ID, constants.RUNNING, "", token)
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

			traceErr := updateJobTrace(jobResponse.ID, log.String(), token)

			if traceErr != nil {
				fmt.Printf("[Job %d] Warning: Trace update failed: %v\n", jobResponse.ID, traceErr)
			}

			if err != nil {
				heartBeat <- true
				fmt.Printf("Script Failure Output: %s\n", string(output))
				footer := fmt.Sprintf("\n\033[31;1mERROR: Command failed: %v\033[0m\n", err)
				log.WriteString(footer)
				_, updateStatusError := updateJobStatus(jobResponse.ID, constants.FAILED, log.String(), token)
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

	_, updateStatusError := updateJobStatus(jobResponse.ID, constants.SUCCESS, log.String(), token)

	if updateStatusError != nil {
		return errorLogger(updateStatusError, "Error Occurred While Updating Job Status")
	}

	return nil
}

func updateJobTrace(jobId int, logContent string, token string) error {

	url := fmt.Sprintf("%s/%d/trace", AppConfig.StatusUpdateURL, jobId)

	req, err := http.NewRequest(constants.PATCH_METHOD, url, strings.NewReader(logContent))
	if err != nil {
		return errorLogger(err, constants.SERIALIZATION_ERROR)
	}

	// Required headers for the Trace API
	req.Header.Set("JOB-TOKEN", token)
	req.Header.Set("Content-Type", "text/plain")
	// Content-Range tells GitLab: "Here is the log from byte 0 to byte X"
	req.Header.Set("Content-Range", fmt.Sprintf("0-%d", len(logContent)))

	resp, err := httpClient.Do(req)
	if err != nil {
		return errorLogger(err, constants.HTTP_REQUEST_ERROR)
	}
	defer resp.Body.Close()

	_, httpResponse := validateResponse(resp)

	if httpResponse != nil {
		return httpResponse
	}

	return nil
}

func updateJobStatus(jobId int, state string, trace string, token string) (int, error) {

	url := AppConfig.StatusUpdateURL

	fullUrl := fmt.Sprintf("%s%d", url, jobId)

	// token := AppConfig.RegistrationToken

	updateJobStatusRequest := UpdateJobStatus{
		Token: token,
		State: state,
		Trace: trace,
	}

	req, err := formHttpRequest(fullUrl, token, updateJobStatusRequest, constants.PUT_METHOD)

	if err != nil {
		return -1, err
	}

	fmt.Printf("--- Invoking Update Job Status API | LOG ->\n %s\n", trace)

	resp, err := httpClient.Do(req)

	if err != nil {
		return -1, errorLogger(err, constants.HTTP_REQUEST_ERROR)
	}

	_, httpError := validateResponse(resp)

	if httpError != nil {
		return resp.StatusCode, httpError
	}

	fmt.Printf("--- Invoking Update Job Status API Completed Successfully | Status Code: %v\n", resp.StatusCode)

	return resp.StatusCode, nil
}

func loadEnv() {

	err := godotenv.Load("../.env")
	if err != nil {
		fmt.Println("Warning: .env file not found, using system env")
	}

	AppConfig = Config{
		RegistrationToken: getEnvOrPanic(constants.TOKEN),
		VerifyURL:         getEnvOrPanic(constants.VERIFY_URL),
		RequestURL:        getEnvOrPanic(constants.REQUEST_URL),
		StatusUpdateURL:   getEnvOrPanic(constants.STATUS_UPDATE_URL),
		SendLogURL:        getEnvOrPanic(constants.STATUS_UPDATE_URL),
	}
}

func getEnvOrPanic(key string) string {
	val := os.Getenv(key)
	if val == "" {
		fmt.Printf("CRITICAL: Environment variable %s is not set\n", key)
		os.Exit(1)
	}
	return val
}

func main() {
	fmt.Println("Go-Runner Process Starts")
	loadEnv()
	healthCheck()
	jobHandler()
}

func jobHandler() {

	jobQueue := make(chan *JobResponse, 10)

	noOfThread := 3 //runtime.NumCpu()
	for i := 0; i < noOfThread; i++ {
		go worker(jobQueue, i+1)
	}

	for {
		job, err := requestJob()

		if err != nil {
			fmt.Println(err)
			time.Sleep(10 * time.Second)
			continue
		}

		if job != nil {
			jobQueue <- job
		} else {
			//fmt.Printf("No Job Found. ")
		}

		time.Sleep(1 * time.Second)
	}
}

func worker(jobQueue <-chan *JobResponse, threadId int) {
	for job := range jobQueue {
		printLog(job.ID, threadId, constants.CLAIMED)
		err := processJob(job)
		if err != nil {
			fmt.Println(err)
		}
		printLog(job.ID, threadId, constants.FINISHED)
	}
}

func errorLogger(err error, message string) error {
	if err != nil {
		return ErrorLogger{
			"Exception Occured: " + message,
			time.Now(),
			err,
		}
	}

	return nil
}

func printLog(jobId int, threadId int, jobType string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] | Thread ID: %d| Successfully %v Job ID: %d\n", timestamp, threadId, jobType, jobId)
}

func validateResponse(resp *http.Response) (bool, error) {

	//fmt.Println("Main Thread | Job Request Response:", resp.StatusCode)

	if resp != nil {

		switch resp.StatusCode {

		case http.StatusOK:
			{
				return true, nil
			}
		case http.StatusNoContent:
			{
				return false, nil
			}
		case http.StatusCreated:
			{
				return true, nil
			}
		default:
			{
				return false, errorLogger(fmt.Errorf(constants.GENERIC_ERROR), constants.HTTP_RESPONSE_ERROR)
			}
		}
	}

	return false, fmt.Errorf("Response code is null")
}

type ErrorLogger struct {
	Message   string
	Timestamp time.Time
	Err       error
}

// Error implements the error interface.
func (e ErrorLogger) Error() string {
	// Handling the missing message case
	msg := e.Message
	if msg == "" {
		msg = "No message provided"
	}

	// Formatting the output: [Time] Context: Original Error
	return fmt.Sprintf("[%s] %s: %v",
		e.Timestamp.Format(time.RFC3339),
		msg,
		e.Err,
	)
}
