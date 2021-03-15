package vk

type apiError struct {
	ErrorCode     int    `json:"error_code"`
	ErrorMsg      string `json:"error_msg"`
	RequestParams []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"request_params"`
}

type ResponseError struct {
	Error *apiError `json:"error"`
}
