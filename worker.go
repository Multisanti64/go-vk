package vk

import (
	"context"
	"go.uber.org/ratelimit"
)

type worker struct {
	transport *transport
	limiter   ratelimit.Limiter
}

func (w *worker) Run(ctx context.Context, requests <-chan *Request) (<-chan string, <-chan *FailedRequest) {
	succeed := make(chan string)
	failed := make(chan *FailedRequest)
	go func() {
		defer close(succeed)
		defer close(failed)
		for {
			select {
			case <-ctx.Done():
				return
			case request := <-requests:
				w.limiter.Take()
				response, fail := w.transport.Send(ctx, request)
				if fail != nil {
					failed <- fail
					continue
				}
				succeed <- response
			}
		}
	}()
	return succeed, failed
}
