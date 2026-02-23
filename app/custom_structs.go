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
	Variables []Variable `json:"variables"`
}

type Variable struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Public bool   `json:"public"`
	Masked bool   `json:"masked"`
}

type UpdateJobStatus struct {
	Token string `json:"token"`
	State string `json:"state"`
	Trace string `json:"trace"`
}

type Config struct {
	RegistrationToken string
	VerifyURL         string
	RequestURL        string
	StatusUpdateURL   string
	SendLogURL        string
}
