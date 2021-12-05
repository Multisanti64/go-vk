package vk

import "strings"

type ApiError struct {
	ErrorCode     int    `json:"error_code"`
	ErrorMsg      string `json:"error_msg"`
	RequestParams []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"request_params"`
}

type ResponseError struct {
	Error *ApiError `json:"error"`
}

func (r *ResponseError) HasRetryErrorCode() bool {
	if r.Error == nil {
		return false
	}
	errorCode := r.Error.ErrorCode
	switch errorCode {
	case 6, 9, 10:
		return true
	}
	return false
}
func (r *ResponseError) HasResponse(value string) bool {
	return strings.Contains(value, "response")
}
func (r *ResponseError) HasErrorCode(value string) bool {
	return strings.Contains(value, "error_code")
}
