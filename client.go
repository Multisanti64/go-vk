package vk

import (
	"context"
	"encoding/json"
	"go.uber.org/ratelimit"
	"math"
	"net/http"
	"sync"
	"time"
)

type Sender interface {
	Send(ctx context.Context, requests []*Request, concurrency int) <-chan string
}
type MethodSender interface {
	Method(ctx context.Context, method Requester, accessToken string, response interface{}) (string, error)
}

type Client struct {
	*http.Client
	v           string
	lang        string
	baseUrl     string
	rateLimiter ratelimit.Limiter
}

func NewClient(v string, lang string) *Client {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	rateLimiter := ratelimit.New(3, ratelimit.Per(1*time.Second))
	return &Client{client, v, lang, "https://api.vk.com/method/", rateLimiter}
}

func (c *Client) SetRateLimiter(rateLimiter ratelimit.Limiter) {
	c.rateLimiter = rateLimiter
}

func (c *Client) SetUrl(url string) {
	c.baseUrl = url
}

func (c *Client) Send(ctx context.Context, requests []*Request, concurrency int) <-chan string {
	maxWorkers := int(math.Min(float64(concurrency), float64(len(requests))))
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	requestsChan := c.makeRequestsChan(requests)
	worker := c.makeWorker()
	var workerResponseChannels []<-chan string
	for i := 0; i < maxWorkers; i++ {
		workerResponseChannels = append(workerResponseChannels, worker.Run(ctx, requestsChan))
	}
	return merge(ctx, workerResponseChannels, maxWorkers)
}

func (c *Client) Method(ctx context.Context, method Requester, accessToken string, response interface{}) (string, error) {
	newContext, cancel := context.WithCancel(ctx)
	defer cancel()
	requests := []*Request{method.ToRequest(accessToken)}
	responseChan := c.Send(newContext, requests, 1)
	vkResponseText := <-responseChan
	err := json.Unmarshal([]byte(vkResponseText), response)
	return vkResponseText, err
}

func (c *Client) makeRequestsChan(requests []*Request) <-chan *Request {
	requestsChan := make(chan *Request)
	go func() {
		defer close(requestsChan)
		for _, r := range requests {
			r.Params.Set("lang", c.lang)
			r.Params.Set("v", c.v)
			requestsChan <- r
		}
	}()
	return requestsChan
}

func (c *Client) makeWorker() Worker {
	worker := Worker{
		client:  c.Client,
		limiter: c.rateLimiter,
		baseUrl: c.baseUrl,
	}
	return worker
}

func merge(ctx context.Context, workerResponseChannels []<-chan string, buffer int) <-chan string {
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
