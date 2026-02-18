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

func formHttpRequest(url string, token string, requestStruct any) (*http.Request, error) {

	//if performacne is needed
	//json.NewEncoder(someWriter).Encode(myStruct).

	jsonData, err := json.Marshal(requestStruct)
	if err != nil {
		return nil, errorLogger(err, constants.SERIALIZATION_ERROR)
	}

	req, err := http.NewRequest(constants.POST_METHOD, url, bytes.NewBuffer(jsonData))
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

	token := AppConfig.Token
	url := AppConfig.VerifyURL

	verifyRunnerRequest := VerifyRequest{
		Token: token,
	}

	req, err := formHttpRequest(url, token, verifyRunnerRequest)

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

	token := AppConfig.Token
	url := AppConfig.RequestURL

	requestJobStruct := JobRequest{
		Token:   token,
		TagList: []string{constants.GOPHER_RUNNER},
	}

	req, err := formHttpRequest(url, token, requestJobStruct)

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

	if deserializationError != nil {
		return nil, errorLogger(deserializationError, constants.DESERIALIZATION_ERROR)
	}

	return &jobResponse, nil
}

func processJob(jobResponse *JobResponse) (err error) {

	var log strings.Builder

	fmt.Printf("--- Job Id: %d\n", jobResponse.ID)

	for i, step := range jobResponse.Steps {
		fmt.Printf("--- Step%d: %v\n", i+1, step.Name)

		stepLog := fmt.Sprintf("\nStep%d: %v \n", i+1, step.Name)
		log.WriteString(stepLog)

		for _, v := range step.Script {
			fmt.Printf("--- Execute Command: %v\n", v)

			shellCmd := exec.Command("bash", "-c", v)
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			// Corrected line:
			header := fmt.Sprintf("\n\033[32;1m[%s] $ %s\033[0m\n", timestamp, v)
			log.WriteString(header)

			output, err := shellCmd.CombinedOutput()

			log.WriteString(string(output))

			if err != nil {
				fmt.Printf("Script Failure Output: %s\n", string(output))
				footer := fmt.Sprintf("\n\033[31;1mERROR: Command failed: %v\033[0m\n", err)
				log.WriteString(footer)
				_, updateStatusError := updateJobStatus(jobResponse.ID, constants.FAILED, log.String())
				if updateStatusError != nil {
					return errorLogger(updateStatusError, "Error Occurred While Updating Job Status")
				}
				return errorLogger(err, "Script Failed With Error")
			}

			fmt.Printf("--- Command Output: %s\n", string(output))
		}
	}

	_, updateStatusError := updateJobStatus(jobResponse.ID, constants.SUCCESS, log.String())

	if err != nil {
		return errorLogger(updateStatusError, "Error Occurred While Updating Job Status")
	}

	return nil
}

func updateJobStatus(jobId int, status string, trace string) (int, error) {

	url := AppConfig.StatusUpdateURL

	fullUrl := fmt.Sprintf("%s%d", url, jobId)

	token := AppConfig.Token

	updateJobStatusRequest := UpdateJobStatus{
		Token:  token,
		Status: status,
		Trace:  trace,
	}

	req, err := formHttpRequest(fullUrl, token, updateJobStatusRequest)

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

	fmt.Printf("--- Invoking Update Job Status API Completed Successfully | Status Code: %v\n", resp)

	return resp.StatusCode, nil
}

func loadEnv() {

	err := godotenv.Load("../.env")
	if err != nil {
		fmt.Println("Warning: .env file not found, using system env")
	}

	AppConfig = Config{
		Token:           getEnvOrPanic(constants.TOKEN),
		VerifyURL:       getEnvOrPanic(constants.VERIFY_URL),
		RequestURL:      getEnvOrPanic(constants.REQUEST_URL),
		StatusUpdateURL: getEnvOrPanic(constants.STATUS_UPDATE_URL),
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

	fmt.Println("Job Request initiates: ")

	for {
		job, err := requestJob()

		if err != nil {
			fmt.Println(err)
			time.Sleep(10 * time.Second)
			continue
		}

		if job != nil {
			fmt.Printf("Successfully claimed Job ID: %d\n", job.ID)
			err := processJob(job)

			if err != nil {
				fmt.Println(err)
			}

			fmt.Printf("Successfully Finished Job | Job ID: %d\n", job.ID)
		} else {
			fmt.Printf("No Job Found. ")
		}

		time.Sleep(10 * time.Second)
	}

	//fmt.Println("Go-Runner Process Ends")

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

func validateResponse(resp *http.Response) (bool, error) {

	fmt.Println("Job Request Response:", resp.StatusCode)

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
