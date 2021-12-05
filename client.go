package vk

import (
	"context"
	"go.uber.org/ratelimit"
	"math"
	"sync"
	"time"
)

type Client struct {
	transport   *transport
	v           string
	lang        string
	rateLimiter ratelimit.Limiter
}

func NewClient(v string, lang string, retryAttempts uint, retryDelay time.Duration) *Client {
	transport := newTransport("https://api.vk.com/method/", retryAttempts, retryDelay)
	rateLimiter := ratelimit.New(3, ratelimit.Per(1*time.Second))
	return &Client{transport, v, lang, rateLimiter}
}

func (c *Client) SetRateLimiter(rateLimiter ratelimit.Limiter) {
	c.rateLimiter = rateLimiter
}

func (c *Client) SendMany(ctx context.Context, requests []*Request, concurrency int) (<-chan string, <-chan *FailedRequest) {
	maxWorkers := int(math.Min(float64(concurrency), float64(len(requests))))
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	requestsChan := c.makeRequestsChan(ctx, requests)
	worker := c.makeWorker()
	var succeedChannels []<-chan string
	var failedChannels []<-chan *FailedRequest
	for i := 0; i < maxWorkers; i++ {
		succeed, failed := worker.Run(ctx, requestsChan)
		succeedChannels = append(succeedChannels, succeed)
		failedChannels = append(failedChannels, failed)
	}
	mergedSucceed := mergeSucceed(ctx, succeedChannels, maxWorkers)
	mergedFailed := mergeFailed(ctx, failedChannels, maxWorkers)
	return mergedSucceed, mergedFailed
}

func (c *Client) SendSingle(ctx context.Context, request *Request) (string, error) {
	response, err := c.transport.Send(ctx, request)
	return response, err
}

func (c *Client) makeRequestsChan(ctx context.Context, requests []*Request) <-chan *Request {
	requestsChan := make(chan *Request)
	go func() {
		defer close(requestsChan)
		for _, r := range requests {
			select {
			case <-ctx.Done():
				return
			default:
				r.Params.Set("lang", c.lang)
				r.Params.Set("v", c.v)
				requestsChan <- r
			}
		}
	}()
	return requestsChan
}

func (c *Client) makeWorker() worker {
	worker := worker{
		transport: c.transport,
		limiter:   c.rateLimiter,
	}
	return worker
}

func mergeSucceed(ctx context.Context, workerResponseChannels []<-chan string, buffer int) <-chan string {
	var wg sync.WaitGroup
	out := make(chan string, buffer)
	output := func(c <-chan string) {
		for response := range c {
			select {
			case <-ctx.Done():
				return
			case out <- response:
			}
		}
		wg.Done()
	}
	wg.Add(len(workerResponseChannels))
	for _, c := range workerResponseChannels {
		go output(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func mergeFailed(ctx context.Context, channels []<-chan *FailedRequest, buffer int) <-chan *FailedRequest {
	var wg sync.WaitGroup
	out := make(chan *FailedRequest, buffer)
	output := func(c <-chan *FailedRequest) {
		for response := range c {
			select {
			case <-ctx.Done():
				return
			case out <- response:
			}
		}
		wg.Done()
	}
	wg.Add(len(channels))
	for _, c := range channels {
		go output(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
