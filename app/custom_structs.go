package main

type VerifyRequest struct {
	Token string `json:"token"`
}

type JobRequest struct {
	Token   string   `json:"token"`
	TagList []string `json:"tag_list"`
}

type JobResponse struct {
	ID    int    `json:"id"`
	Token string `json:"token"`
	Steps []struct {
		Name   string   `json:"name"`
		Script []string `json:"script"`
	} `json:"steps"`
}

type UpdateJobStatus struct {
	Token  string `json:"token"`
	Status string `json:"status"`
	Trace  string `json:"trace"`
}

type Config struct {
	Token           string
	VerifyURL       string
	RequestURL      string
	StatusUpdateURL string
}
