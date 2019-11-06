package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/jetrtc/log"
)

type Auth interface {
	Authorize(req *http.Request) error
	Validate(res *http.Response) (bool, error)
	Invalidate() error
}

type Response struct {
	*http.Response
	Body     []byte
	protobuf bool
}

func (r *Response) Unmarshal(val interface{}) error {
	var err error
	protobuf := false
	switch val := val.(type) {
	case proto.Message:
		if r.protobuf {
			protobuf = true
			err = proto.Unmarshal(r.Body, val)
		} else {
			err = json.Unmarshal(r.Body, val)
		}
	default:
		err = json.Unmarshal(r.Body, val)
	}
	if err != nil && !protobuf && r.Response.Header.Get("Content-Type") != "application/json" {
		err = fmt.Errorf("%s", r.Body)
	}
	return err
}

type Client struct {
	log.Loggable
	client   *http.Client
	auth     Auth
	protobuf bool
}

func NewClient(logger log.Logger, timeout time.Duration) *Client {
	return &Client{
		Loggable: log.Loggable{
			Logger: logger,
		},
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) Auth(auth Auth) *Client {
	c.auth = auth
	return c
}

func (c *Client) Protobuf() *Client {
	c.protobuf = true
	return c
}

func (c *Client) Get(url string) (*Response, error) {
	return c.Request("GET", url, nil)
}

func (c *Client) Post(url string, req interface{}) (*Response, error) {
	return c.Request("POST", url, req)
}

func (c *Client) Put(url string, req interface{}) (*Response, error) {
	return c.Request("PUT", url, req)
}

func (c *Client) Delete(url string) (*Response, error) {
	return c.Request("DELETE", url, nil)
}

func (c *Client) Request(method, url string, r interface{}) (*Response, error) {
	start := time.Now()
	res, err := c.request(method, url, r)
	if err != nil {
		return nil, err
	}
	if c.auth != nil {
		valid, err := c.auth.Validate(res.Response)
		if err != nil {
			c.Errorf("Failed to validate auth: %s", err.Error())
			return nil, err
		}
		if !valid {
			err = c.auth.Invalidate()
			if err != nil {
				c.Errorf("Failed to invalidate auth: %s", err.Error())
				return nil, err
			}
			res, err = c.request(method, url, r)
			if err != nil {
				return nil, err
			}
		}
	}
	c.Infof("Requested in %v: %s %s", time.Now().Sub(start), method, url)
	return res, nil
}

func (c *Client) request(method, url string, r interface{}) (*Response, error) {
	var body []byte
	var err error
	protobuf := false
	if r != nil {
		switch r := r.(type) {
		case proto.Message:
			if c.protobuf {
				body, err = proto.Marshal(r)
				protobuf = true
			} else {
				body, err = json.Marshal(r)
			}
		default:
			body, err = json.Marshal(r)
		}
		if err != nil {
			c.Errorf("Failed to marshal: %s", err.Error())
			return nil, err
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		c.Errorf("Failed to create request: %s", err.Error())
		return nil, err
	}
	if c.auth != nil {
		auth := c.auth
		c.auth = nil
		err := auth.Authorize(req)
		if err != nil {
			c.auth = auth
			c.Errorf("Failed to authorize: %s", err.Error())
			return nil, err
		}
		c.auth = auth
	}
	if body != nil && len(body) > 0 {
		if protobuf {
			req.Header.Set("Content-Type", "application/protobuf")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	} else {
		if c.protobuf {
			req.Header.Set("Accept", "application/protobuf")
		} else {
			req.Header.Set("Accept", "application/json")
		}
	}
	c.dumpRequest(req, r, body)
	res, err := c.client.Do(req)
	if err != nil {
		c.Errorf("Failed to make request: %s", err.Error())
		return nil, err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		c.Errorf("Failed to read body: %s", err.Error())
		return nil, err
	}
	c.dumpResponse(res, data, nil)
	res.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	return &Response{
		Response: res,
		Body:     data,
		protobuf: c.protobuf,
	}, nil
}

func (c *Client) dumpRequest(req *http.Request, v interface{}, data []byte) {
	dump := &struct {
		Method   string                 `json:"method"`
		URL      string                 `json:"url"`
		Protocol string                 `json:"protocol"`
		Headers  map[string]interface{} `json:"headers"`
		Body     interface{}            `json:"body,omitempty"`
		Data     []byte                 `json:"data,omitempty"`
	}{
		Method:   req.Method,
		URL:      req.URL.RequestURI(),
		Protocol: req.Proto,
		Headers:  make(map[string]interface{}),
		Body:     v,
		Data:     data,
	}
	for k, v := range req.Header {
		if len(v) == 1 {
			dump.Headers[k] = v[0]
		} else {
			dump.Headers[k] = v
		}
	}
	bytes, err := json.Marshal(dump)
	if err != nil {
		c.Debugf("%s", bytes)
	}
}

func (c *Client) dumpResponse(res *http.Response, data []byte, v interface{}) {
	dump := &struct {
		Status   string                 `json:"status"`
		Protocol string                 `json:"protocol"`
		Headers  map[string]interface{} `json:"headers"`
		Body     interface{}            `json:"body,omitempty"`
		Data     []byte                 `json:"data,omitempty"`
	}{
		Status:   res.Status,
		Protocol: res.Proto,
		Headers:  make(map[string]interface{}),
		Data:     data,
	}
	ctype := res.Header.Get("Content-Type")
	if v != nil && (strings.HasPrefix(ctype, "application/json") || strings.HasPrefix(ctype, "application/protobuf")) {
		dump.Body = v
	} else {
		dump.Body = string(data)
	}
	for k, v := range res.Header {
		if len(v) == 1 {
			dump.Headers[k] = v[0]
		} else {
			dump.Headers[k] = v
		}
	}
	bytes, err := json.Marshal(dump)
	if err != nil {
		c.Debugf("%s", bytes)
	}
}
