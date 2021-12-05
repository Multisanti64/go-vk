package vk

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"io/ioutil"
	"net/http"
	"time"
)

type FailedRequest struct {
	request       *Request
	response      string
	responseError *ResponseError
	info          string
}

func (r *FailedRequest) Error() string {
	return r.response
}

type transport struct {
	baseUrl      string
	httpClient   *http.Client
	retryOptions []retry.Option
}

func newTransport(baseUrl string, retryAttempts uint, retryDelay time.Duration) *transport {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	retryOptions := []retry.Option{
		retry.Attempts(retryAttempts),
		retry.Delay(retryDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
	}
	t := &transport{
		baseUrl:      baseUrl,
		httpClient:   httpClient,
		retryOptions: retryOptions,
	}
	return t
}

func (t *transport) Send(ctx context.Context, request *Request) (response string, failedRequest *FailedRequest) {
	sendRequest := func() error {
		r, err := t.doRequest(ctx, request)
		response = r
		if err != nil {
			return &FailedRequest{
				request:       request,
				response:      response,
				responseError: nil,
				info:          err.Error(),
			}
		}
		responseError, err := t.getErrorFromResponse(response)
		if err != nil {
			failedRequest = &FailedRequest{
				request:       request,
				response:      response,
				responseError: nil,
				info:          err.Error(),
			}
			return nil
		}
		if responseError != nil {
			if responseError.HasRetryErrorCode() {
				return &FailedRequest{
					request:       request,
					response:      response,
					responseError: responseError,
					info:          "rate limit",
				}
			}
			failedRequest = &FailedRequest{
				request:       request,
				response:      response,
				responseError: responseError,
			}
			return nil
		}
		return nil
	}
	err := retry.Do(sendRequest, append(t.retryOptions, retry.Context(ctx))...)
	if err != nil {
		failedRequest, ok := err.(*FailedRequest)
		if ok {
			return response, failedRequest
		}
	}
	return response, failedRequest
}

func (t *transport) doRequest(ctx context.Context, request *Request) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, request.getFullUrl(t.baseUrl), request.getParams())
	resp, err := t.httpClient.Do(req)
	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
	if err != nil {
		return err.Error(), err
	}
	body, _ := ioutil.ReadAll(resp.Body)
	result := string(body)
	if resp.StatusCode != 200 {
		return result, fmt.Errorf("expected status code 200, got %d", resp.StatusCode)
	}
	return result, nil
}

func (t *transport) getErrorFromResponse(response string) (*ResponseError, error) {
	responseError := &ResponseError{}
	if responseError.HasErrorCode(response) {
		err := json.Unmarshal([]byte(response), responseError)
		if err != nil {
			return nil, err
		}
		if responseError.Error != nil {
			return responseError, nil
		}
	}
	return nil, nil
}
