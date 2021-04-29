package vk

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"go.uber.org/ratelimit"
	"io/ioutil"
	"net/http"
	"time"
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
				rawResponse, err := w.send(request)
				lastRawResponse = rawResponse
				if err != nil {
					return err
				}
				var response ResponseError
				err = json.Unmarshal([]byte(rawResponse), &response)
				if err != nil {
					return nil
				}
				if hasRetryErrorCode(&response) {
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
			err := retry.Do(sendResponse(request), retry.Attempts(3), retry.Delay(1 * time.Second), retry.DelayType(retry.BackOffDelay), retry.Context(ctx))
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
		return err.Error(), err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, _ := ioutil.ReadAll(resp.Body)
	result := string(body)
	err = resp.Body.Close()
	if err != nil {
		return result, err
	}
	return result, nil
}

func hasRetryErrorCode(response *ResponseError) bool {
	if response.Error == nil {
		return false
	}
	errorCode := response.Error.ErrorCode
	switch errorCode {
	case 6, 9, 10:
		return true
	}
	return false
}
