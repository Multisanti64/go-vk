package vk

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"net/url"
	"testing"
	"time"
)

func TestSend(t *testing.T) {
	defer gock.Off()

	type ValidResponseData struct {
		Count int   `json:"count"`
		Items []int `json:"items"`
	}
	type ValidResponse struct {
		Response ValidResponseData `json:"response"`
	}

	validResponse := ValidResponse{
		Response: ValidResponseData{
			Count: 4,
			Items: []int{1, 2, 3, 4},
		},
	}

	want, _ := json.Marshal(validResponse)

	gock.New("https://api.vk.com/method").
		Post("/users.get").
		Reply(200).
		JSON(validResponse)

	ctx := context.Background()
	request := &Request{
		Method: "users.get",
		Params: url.Values{"user_id": {"1"}},
	}
	sut := newTransport("https://api.vk.com/method/", 1, 1*time.Second)
	response, failedRequest := sut.Send(ctx, request)
	assert.Nil(t, failedRequest, "no error")
	assert.JSONEq(t, string(want), response, "got expected json")
}

func TestHttpErrorAndRetries(t *testing.T) {
	defer gock.Off()
	attempts := 3
	mock := gock.New("https://api.vk.com/method").
		Post("/users.get").
		Times(attempts).
		Reply(500)
	sut := newTransport("https://api.vk.com/method/", uint(attempts), 1*time.Microsecond)
	ctx := context.Background()
	request := &Request{
		Method: "users.get",
		Params: url.Values{"user_id": {"1"}},
	}
	_, failedRequest := sut.Send(ctx, request)
	assert.True(t, mock.Done(), "expected mock to be done, after all attempts")
	assert.NotNil(t, failedRequest, "expected to return failed request")
}

func TestTooManyRequests(t *testing.T) {

	attempts := 3
	tooManyRequests := ResponseError{Error: &ApiError{
		ErrorCode:     6,
		ErrorMsg:      "Too many requests",
		RequestParams: nil,
	}}

	defer gock.Off()
	mock := gock.New("https://api.vk.com/method").
		Post("/users.get").
		Times(attempts).
		Reply(200).
		JSON(tooManyRequests).JSON(tooManyRequests).JSON(tooManyRequests)

	sut := newTransport("https://api.vk.com/method/", uint(attempts), 1*time.Microsecond)
	ctx := context.Background()
	request := &Request{
		Method: "users.get",
		Params: url.Values{"user_id": {"1"}},
	}
	_, failedRequest := sut.Send(ctx, request)
	assert.True(t, mock.Done(), "expected mock to be done, after all attempts")
	assert.NotNil(t, failedRequest, "expected to return failed request")
	assert.NotNil(t, failedRequest.responseError, "expected have responseError")
	assert.Equal(t, tooManyRequests.Error.ErrorCode, failedRequest.responseError.Error.ErrorCode, "expected error codes to match")
}

func TestVkError(t *testing.T) {
	fakeError := ResponseError{Error: &ApiError{
		ErrorCode:     200,
		ErrorMsg:      "Fake error",
		RequestParams: nil,
	}}

	defer gock.Off()
	gock.New("https://api.vk.com/method").
		Post("/users.get").
		Reply(200).
		JSON(fakeError)

	sut := newTransport("https://api.vk.com/method/", 3, 1*time.Microsecond)
	ctx := context.Background()
	request := &Request{
		Method: "users.get",
		Params: url.Values{"user_id": {"1"}},
	}
	_, failedRequest := sut.Send(ctx, request)

	assert.NotNil(t, failedRequest, "expected to return failed request")
	assert.NotNil(t, failedRequest.responseError, "expected have responseError")
	assert.Equal(t, fakeError.Error.ErrorCode, failedRequest.responseError.Error.ErrorCode, "expected error codes to match")
}
