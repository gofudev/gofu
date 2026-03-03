package http

import (
	"context"
	"io"
	"net"
	stdhttp "net/http"
	"strings"
	"time"
)

type Request struct {
	Method  string
	URL     string
	Headers map[string][]string
	Body    string
}

type Response struct {
	StatusCode int
	Headers    map[string][]string
	Body       string
}

type defaultResolver struct{}

func (defaultResolver) LookupIPAddr(host string) ([]net.IP, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, len(addrs))
	for i, a := range addrs {
		ips[i] = a.IP
	}
	return ips, nil
}

var client = &stdhttp.Client{
	Timeout: 10 * time.Second,
	Transport: &stdhttp.Transport{
		DialContext: (&safeDialer{resolver: defaultResolver{}}).DialContext,
	},
}

func Do(req Request) (Response, error) {
	method := req.Method
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	stdReq, err := stdhttp.NewRequest(method, req.URL, bodyReader)
	if err != nil {
		return Response{}, err
	}

	for k, vs := range req.Headers {
		for _, v := range vs {
			stdReq.Header.Add(k, v)
		}
	}

	resp, err := client.Do(stdReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}

	return Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       string(body),
	}, nil
}

func Get(url string) (Response, error) {
	return Do(Request{Method: "GET", URL: url})
}

func Post(url string, contentType string, body string) (Response, error) {
	return Do(Request{
		Method:  "POST",
		URL:     url,
		Headers: map[string][]string{"Content-Type": {contentType}},
		Body:    body,
	})
}
