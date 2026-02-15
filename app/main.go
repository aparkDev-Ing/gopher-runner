package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	constants "gitlab.com/aparkdev-ing/gopher-runner/constants"
)

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

	jsonData, err := json.Marshal(requestStruct)
	if err != nil {
		return nil, errorLogger(err, constants.SERIALIZATION_ERROR)
	}

	req, err := http.NewRequest(constants.POST_METHOD, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errorLogger(err, constants.HTTP_REQUEST_FORM_ERROR)
	}

	return req, nil
}

func createHttpClient(httpRequest *http.Request) *http.Client {

	httpRequest.Header.Set(constants.CONTENT_TYPE, constants.JSON)

	return &http.Client{
		Timeout: 20 * time.Second,
	}
}

func verifyRunner() (int, error) {

	token := os.Getenv(constants.TOKEN)
	url := os.Getenv(constants.VERIFY_URL)

	verifyRunnerRequest := VerifyRequest{
		Token: token,
	}

	req, err := formHttpRequest(url, token, verifyRunnerRequest)

	if err != nil {
		return -1, err
	}

	client := createHttpClient(req)

	resp, err := client.Do(req)

	if err != nil {
		return -1, errorLogger(err, constants.HTTP_REQUEST_ERROR)
	}

	statusCode := resp.StatusCode

	defer resp.Body.Close()
	return statusCode, nil
}

func requestJob() (*JobResponse, error) {

	token := os.Getenv(constants.TOKEN)
	url := os.Getenv(constants.REQUEST_URL)

	fmt.Println("request url:", url)

	requestJobStruct := JobRequest{
		Token:   token,
		TagList: []string{constants.GOPHER_RUNNER},
	}

	req, err := formHttpRequest(url, token, requestJobStruct)

	if err != nil {
		return nil, err
	}

	client := createHttpClient(req)

	resp, err := client.Do(req)

	if err != nil {
		return nil, errorLogger(err, constants.HTTP_REQUEST_ERROR)
	}

	defer resp.Body.Close()

	// if resp != nil && resp.StatusCode == http.StatusNoContent {
	// 	fmt.Println("Job Request Response 204:", resp)
	// 	return nil, nil
	// }

	// if resp != nil && resp.StatusCode != http.StatusCreated {
	// 	return nil, errorLogger(fmt.Errorf(constants.GENERIC_ERROR), constants.HTTP_REQUEST_ERROR)
	// }

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

func main() {
	fmt.Println("Go-Runner Process Starts")
	godotenv.Load("../.env")

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

	if resp != nil && resp.StatusCode != 0 {
		switch resp.StatusCode {
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
				return false, errorLogger(fmt.Errorf(constants.GENERIC_ERROR), constants.HTTP_REQUEST_ERROR)
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
