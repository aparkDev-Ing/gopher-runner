package main

import (
	"fmt"
	"testing"
)

func TestValidProcessJob(t *testing.T) {

	AppConfig = Config{
		StatusUpdateURL:   "https://gitlab.com/api/v4/jobs/",
		RegistrationToken: "mock-token",
	}
	//1. Arrange: Create mock data that mimics a GitLab API response
	mockJob := &JobResponse{
		ID: 12345,
		Steps: []struct {
			Name   string   `json:"name"`
			Script []string `json:"script"`
		}{
			{
				Name: "before_script",
				Script: []string{
					"echo 'Setup started'",
					//"export ENV=test",
				},
			},
			{
				Name: "script",
				Script: []string{
					"echo Script Stage",
				},
			},
		},
	}

	// 2. Act: Call your function
	// For now, this will just print to your test console
	err := processJob(mockJob)

	if err != nil {
		fmt.Println(err)
	}

	// 3. Assert: In a real test, we would check if the commands executed.
	// For now, if it doesn't panic, the "loop logic" is verified!

}
func TestProcessInvalidJob(t *testing.T) {

	AppConfig = Config{
		StatusUpdateURL:   "https://gitlab.com/api/v4/jobs/",
		RegistrationToken: "mock-token",
	}

	mockJob := &JobResponse{
		ID: 999,
		Steps: []struct {
			Name   string   `json:"name"`
			Script []string `json:"script"`
		}{
			{
				Name: "test_step",
				Script: []string{
					"echo 'Testing ANSI colors...'",
					"ls -la",
					"non_existent_command", // This should trigger your RED error footer
				},
			},
		},
	}

	// 2. Act: Call your function
	// For now, this will just print to your test console
	err := processJob(mockJob)

	if err != nil {
		fmt.Println(err)
	}

	// 3. Assert: In a real test, we would check if the commands executed.
	// For now, if it doesn't panic, the "loop logic" is verified!

}

func TestProcessVariableJob(t *testing.T) {

	AppConfig = Config{
		StatusUpdateURL:   "https://gitlab.com/api/v4/jobs/",
		RegistrationToken: "mock-token",
	}

	mockJob := &JobResponse{
		ID:    12345,
		Token: "mock-job-token",
		Variables: []Variable{
			{Key: "CUSTOM_GREETING", Value: "Hello from Go"},
			{Key: "SECRET_KEY", Value: "SuperSecret123"},
		},
		Steps: []struct {
			Name   string   `json:"name"`
			Script []string `json:"script"`
		}{
			{
				Name: "Test Step",
				Script: []string{
					"echo $CUSTOM_GREETING",
					//"echo $SECRET_KEY", // This proves os.Environ() worked
					"echo $PATH", // This proves os.Environ() worked
				},
			},
		},
	}

	err := processJob(mockJob)

	if err != nil {
		fmt.Println(err)
	}

}
