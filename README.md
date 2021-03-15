# go-vk
Simple golang client for vk.com api

## Features
* Rate limiting
* Auto retry on rate limit
* Concurrent execution
* Context support

## Usage
Make client with `client := vk.NewClient("5.103", "ru")`
Send request and read from channel
```
requests := []*vk.Request{{
	Method: "users.get",
	Params: url.Values{"access_token":  {"foo"}},
}}
resultChannel := p.client.Send(ctx, requests, 1)
response := <- resultChannel
```  
