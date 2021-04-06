package vk

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"go.uber.org/ratelimit"
	"io/ioutil"
	"net/http"
)

type Worker struct {
	client  *http.Client
	limiter ratelimit.Limiter
	baseUrl string
}

func (w *Worker) Run(ctx context.Context, requests <-chan *Request) <-chan string {
	outChanResponses := make(chan string)
	go func() {
		defer close(outChanResponses)
		var lastRawResponse string
		sendResponse := func(request *Request) func() error {
			return func() error {
				w.limiter.Take()
				rawResponse, _ := w.send(request)
				lastRawResponse = rawResponse
				var response ResponseError
				err := json.Unmarshal([]byte(rawResponse), &response)
				if err != nil {
					return nil
				}
				if isTooManyAttempts(&response) {
					return fmt.Errorf("too many attempts")
				}
				outChanResponses <- rawResponse
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return
		case request := <-requests:
			err := retry.Do(sendResponse(request), retry.Attempts(3), retry.Delay(0), retry.Context(ctx))
			if err != nil {
				outChanResponses <- lastRawResponse
			}
		}
	}()
	return outChanResponses
}

func (w *Worker) send(request *Request) (string, error) {
	req, _ := http.NewRequest(http.MethodPost, request.getFullUrl(w.baseUrl), request.getParams())
	resp, err := w.client.Do(req)
	if err != nil {
		panic(err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	result := string(body)
	err = resp.Body.Close()
	if err != nil {
		return result, err
	}
	return result, nil
}

func isTooManyAttempts(response *ResponseError) bool {
	return response.Error != nil && (response.Error.ErrorCode == 6 || response.Error.ErrorCode == 9)
}
