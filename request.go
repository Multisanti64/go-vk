package vk

import (
	"io"
	"net/url"
	"strings"
)

type Requester interface {
	ToRequest(accessToken string) *Request
}

type Request struct {
	Method string
	Params url.Values
}

func (r *Request) getFullUrl(base string) string {
	var stringBuilder strings.Builder
	stringBuilder.WriteString(base)
	stringBuilder.WriteString(r.Method)
	fullUrl := stringBuilder.String()
	return fullUrl
}

func (r *Request) getParams() io.Reader {
	return strings.NewReader(r.Params.Encode())
}
