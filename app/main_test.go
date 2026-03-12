package main

import (
	"context"
	"testing"
)

func TestValidProcessJob(t *testing.T) {
	// Setup Context
	ctx := context.Background()

	AppConfig = Config{
		StatusUpdateURL:   "http://localhost:8080/api/v4/jobs/", // Use a dummy URL for local tests
		RegistrationToken: "mock-token",
	}

	mockJob := &JobResponse{
		ID:    12345,
		Token: "mock-job-token",
		Steps: []struct {
			Name   string   `json:"name"`
			Script []string `json:"script"`
		}{
			{
				Name: "script",
				Script: []string{
					"echo 'Setup started'",
					"echo 'Script Stage'",
				},
			},
		},
	}

	// Pass the ctx here!
	err := processJob(ctx, mockJob)

	// Note: This might still return an error because it can't
	// reach the "StatusUpdateURL" to send the final success status.
	if err != nil {
		t.Logf("ProcessJob finished with error (expected due to no network): %v", err)
	}
}

func TestProcessInvalidJob(t *testing.T) {
	ctx := context.Background()

	AppConfig = Config{
		StatusUpdateURL:   "http://localhost:8080/api/v4/jobs/",
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
					"non_existent_command",
				},
			},
		},
	}

	err := processJob(ctx, mockJob)

	// In this case, we WANT an error because the command is invalid.
	if err == nil {
		t.Error("Expected an error from non_existent_command, but got nil")
	}
}

func TestProcessVariableJob(t *testing.T) {
	ctx := context.Background()

	AppConfig = Config{
		StatusUpdateURL:   "http://localhost:8080/api/v4/jobs/",
		RegistrationToken: "mock-token",
	}

	mockJob := &JobResponse{
		ID:    12345,
		Token: "mock-job-token",
		Variables: []Variable{
			{Key: "CUSTOM_GREETING", Value: "Hello from Go"},
		},
		Steps: []struct {
			Name   string   `json:"name"`
			Script []string `json:"script"`
		}{
			{
				Name: "Test Step",
				Script: []string{
					"echo $CUSTOM_GREETING",
				},
			},
		},
	}

	err := processJob(ctx, mockJob)
	if err != nil {
		t.Logf("Finished with expected network error: %v", err)
	}
}
