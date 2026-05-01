package util

import (
	"net/http"

	"golang.org/x/time/rate"
)

type RateLimitRoundTripper struct {
	Transport http.RoundTripper
	Limiter   *rate.Limiter
}

func (r *RateLimitRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	err := r.Limiter.Wait(req.Context())
	if err != nil {
		return nil, err
	}
	return r.Transport.RoundTrip(req)
}
