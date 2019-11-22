package rest

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jetrtc/log"
)

type HandlerFunc func(s *Session)

type MiddlewareFunc func(handler HandlerFunc) HandlerFunc

type route struct {
	server      *Server
	handlerFunc HandlerFunc
}

func (rt *route) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer io.Copy(ioutil.Discard, r.Body)
	}
	s := &Session{
		Context:        log.NewContext(r.Context(), rt.server),
		server:         rt.server,
		Data:           make(map[interface{}]interface{}),
		Request:        r,
		ResponseWriter: w,
	}
	handler := rt.handlerFunc
	for _, mw := range rt.server.middlewares {
		handler = mw(handler)
	}
	defer func() {
		for k := range s.Data {
			delete(s.Data, k)
		}
	}()
	handler(s)
}

type Server struct {
	log.Sugar
	jsonPrefix, jsonIndent string
	middlewares            []MiddlewareFunc
}

func NewServer(logger log.Logger) *Server {
	return &Server{
		Sugar:       log.NewSugar(logger),
		middlewares: make([]MiddlewareFunc, 0),
	}
}

func (s *Server) JSONIndent(prefix, indent string) {
	s.jsonPrefix = prefix
	s.jsonIndent = indent
}

func (s *Server) Post(r *mux.Route, handler HandlerFunc) *mux.Route {
	return r.Handler(s.HandlerFunc(handler)).Methods("POST")
}

func (s *Server) Get(r *mux.Route, handler HandlerFunc) *mux.Route {
	return r.Handler(s.HandlerFunc(handler)).Methods("GET")
}

func (s *Server) Put(r *mux.Route, handler HandlerFunc) *mux.Route {
	return r.Handler(s.HandlerFunc(handler)).Methods("PUT")
}

func (s *Server) Delete(r *mux.Route, handler HandlerFunc) *mux.Route {
	return r.Handler(s.HandlerFunc(handler)).Methods("DELETE")
}

func (s *Server) HandlerFunc(handler HandlerFunc) http.Handler {
	return &route{
		server:      s,
		handlerFunc: handler,
	}
}

func (s *Server) Use(middlewares ...MiddlewareFunc) *Server {
	s.middlewares = append(middlewares, s.middlewares...)
	return s
}
