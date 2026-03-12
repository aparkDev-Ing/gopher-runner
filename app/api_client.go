package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	constants "gitlab.com/aparkdev-ing/gopher-runner/constants"
)

var httpClient *http.Client = &http.Client{
	Timeout: 60 * time.Second,
}

func healthCheck(ctx context.Context) {

	fmt.Println("Verification Starts")

	resp, err := verifyRunner(ctx)

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

func formHttpRequest(ctx context.Context, url string, token string, requestStruct any, httpMethod string) (*http.Request, error) {

	//if performacne is needed
	//json.NewEncoder(someWriter).Encode(myStruct).

	jsonData, err := json.Marshal(requestStruct)
	if err != nil {
		return nil, errorLogger(err, constants.SERIALIZATION_ERROR)
	}

	//fmt.Printf("DEBUG JSON: %s\n", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, httpMethod, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errorLogger(err, constants.HTTP_REQUEST_FORM_ERROR)
	}

	req.Header.Set(constants.CONTENT_TYPE, constants.JSON)

	return req, nil
}

func verifyRunner(ctx context.Context) (int, error) {

	token := AppConfig.RegistrationToken
	url := AppConfig.VerifyURL

	verifyRunnerRequest := VerifyRequest{
		Token: token,
	}

	req, err := formHttpRequest(ctx, url, token, verifyRunnerRequest, constants.POST_METHOD)

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

func requestJob(ctx context.Context) (*JobResponse, error) {

	token := AppConfig.RegistrationToken
	url := AppConfig.RequestURL

	requestJobStruct := JobRequest{
		Token:              token,
		TagList:            []string{constants.GOPHER_RUNNER},
		LongPollingTimeout: 50,
	}

	req, err := formHttpRequest(ctx, url, token, requestJobStruct, constants.POST_METHOD)

	if err != nil {
		return nil, err
	}

	req.Header.Set("X-GitLab-Runner-Poll-Config", "long_polling_timeout=50")

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

func updateJobStatus(ctx context.Context, jobId int, state string, trace string, token string) (int, error) {

	url := AppConfig.StatusUpdateURL

	fullUrl := fmt.Sprintf("%s%d", url, jobId)

	// token := AppConfig.RegistrationToken

	updateJobStatusRequest := UpdateJobStatus{
		Token: token,
		State: state,
		Trace: trace,
	}

	req, err := formHttpRequest(ctx, fullUrl, token, updateJobStatusRequest, constants.PUT_METHOD)

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

func updateJobTrace(jobId int, logContent string, offset int, token string) error {

	url := fmt.Sprintf("%s/%d/trace", AppConfig.StatusUpdateURL, jobId)

	req, err := http.NewRequest(constants.PATCH_METHOD, url, strings.NewReader(logContent))
	if err != nil {
		return errorLogger(err, constants.SERIALIZATION_ERROR)
	}

	lastByte := offset + len(logContent) - 1
	req.Header.Set("Content-Range", fmt.Sprintf("%d-%d", offset, lastByte))
	req.Header.Set("JOB-TOKEN", token)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := httpClient.Do(req)
	if err != nil {
		return errorLogger(err, constants.HTTP_REQUEST_ERROR)
	}
	defer resp.Body.Close()

	//fmt.Println("Response Code:", resp.StatusCode)

	// bodyBytes, _ := io.ReadAll(resp.Body)
	// fmt.Println("Raw response:", string(bodyBytes))

	_, httpResponse := validateResponse(resp)

	if httpResponse != nil {
		return httpResponse
	}

	return nil
}

func validateResponse(resp *http.Response) (bool, error) {

	// ... inside requestJob ...
	// bodyBytes, _ := io.ReadAll(resp.Body)
	// fmt.Println("Raw JSON Response:", string(bodyBytes))

	if resp != nil {

		switch resp.StatusCode {

		case http.StatusAccepted:
			{
				return true, nil
			}
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
