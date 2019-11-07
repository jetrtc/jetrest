package rest

import (
	"net/url"
	"strings"
)

type URL struct {
	url   string
	query url.Values
}

func NewURL(u string) *URL {
	return &URL{url: u, query: make(url.Values)}
}

func (r *URL) Join(path string) *URL {
	if strings.HasSuffix(r.url, "/") {
		if strings.HasPrefix(path, "/") {
			r.url += path[1:]
		} else {
			r.url += path
		}
	} else {
		if strings.HasPrefix(path, "/") {
			r.url += path
		} else {
			r.url += "/" + path
		}
	}
	return r
}

func (r *URL) Param(name, value string) *URL {
	sub := "{" + name + "}"
	i := strings.Index(r.url, sub)
	if i > 0 {
		r.url = r.url[:i] + value + r.url[i+len(sub):]
	} else {
		r.query.Add(name, value)
	}
	return r
}

func (r *URL) Encode() string {
	if len(r.query) > 0 {
		return r.url + "?" + r.query.Encode()
	} else {
		return r.url
	}
}
