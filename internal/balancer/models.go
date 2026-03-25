package balancer

import (

)

type ServerResponse struct {
	Address string `json:"address"`
	Healthy bool   `json:"healthy"`
}

type RegisterServerRequest struct {
	Addr string `json:"addr"`
}

type MetricsResponse struct {
	Requests  int64 `json:"requests"`
	Successes int64 `json:"successes"`
	Failures  int64 `json:"failures"`
}