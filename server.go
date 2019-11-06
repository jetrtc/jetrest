package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/jetrtc/log"
)

const (
	ContentType     = "Content-Type"
	JsonContentType = "application/json"
)

var (
	ProtobufContentTypes = []string{"application/protobuf", "application/x-protobuf"}
)

type Session struct {
	*log.Context
	Request        *http.Request
	statusCode     int
	responseWriter http.ResponseWriter
}

func (s *Session) Header() http.Header {
	return s.responseWriter.Header()
}

func (s *Session) Write(data []byte) (int, error) {
	return s.responseWriter.Write(data)
}

func (s *Session) WriteHeader(statusCode int) {
	if s.statusCode == 0 {
		s.responseWriter.WriteHeader(statusCode)
		s.statusCode = statusCode
	} else {
		s.Warningf("Attemp to write header again: %d", statusCode)
	}
}

func (s *Session) Decode(val interface{}) error {
	switch v := val.(type) {
	case proto.Message:
		if isProto(contentType(s.Request)) {
			data, err := ioutil.ReadAll(s.Request.Body)
			if err != nil {
				return err
			}
			return proto.Unmarshal(data, v)
		} else {
			err := json.NewDecoder(s.Request.Body).Decode(v)
			if err != nil {
				return err
			}
			// valid data if decoded by json
			_, err = proto.Marshal(v)
			return err
		}
	default:
		return json.NewDecoder(s.Request.Body).Decode(v)
	}
}

func (s *Session) Vars() map[string]string {
	return mux.Vars(s.Request)
}

func (s *Session) Var(key, preset string) string {
	val := s.Vars()[key]
	if val == "" {
		val = s.Request.FormValue(key)
	}
	if val == "" {
		val = preset
	}
	return val
}

type HandlerFunc func(s *Session) (res interface{})

type MiddlewareFunc func(handler HandlerFunc) HandlerFunc

type route struct {
	server      *Server
	handlerFunc HandlerFunc
}

func (rt *route) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s := &Session{
		Context:        log.NewContext(rt.server, r.Context()),
		Request:        r,
		responseWriter: w,
	}
	handler := rt.handlerFunc
	for _, mw := range rt.server.middlewares {
		handler = mw(handler)
	}
	res := handler(s)
	if r.Body != nil {
		defer io.Copy(ioutil.Discard, r.Body)
	}
	writeProto := func(v proto.Message, accept string) error {
		if accept == "" {
			accept = ProtobufContentTypes[0]
		}
		w.Header().Set(ContentType, accept)
		data, err := proto.Marshal(v)
		if err != nil {
			s.Errorf("Failed to encode protobuf: %s", err.Error())
			return err
		}
		_, err = io.Copy(w, bytes.NewBuffer(data))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		s.Debugf("%s %s => %d: %s", r.Method, r.URL.Path, s.statusCode, accept)
		return nil
	}
	writeJSON := func(v interface{}) error {
		w.Header().Set(ContentType, JsonContentType)
		if rt.server.jsonIndent != "" || rt.server.jsonPrefix != "" {
			data, err := json.MarshalIndent(v, rt.server.jsonPrefix, rt.server.jsonIndent)
			if err != nil {
				s.Errorf("Failed to encode JSON: %s", err.Error())
				return err
			}
			_, err = w.Write(data)
			if err != nil {
				s.Errorf("Failed to write JSON: %s", err.Error())
				return err
			}
		} else {
			err := json.NewEncoder(w).Encode(v)
			if err != nil {
				s.Errorf("Failed to encode JSON: %s", err.Error())
				return err
			}
		}
		s.Debugf("%s %s => %d: %s", r.Method, r.URL.Path, s.statusCode, JsonContentType)
		return nil
	}
	err := func() error {
		if notNil(res) {
			switch v := res.(type) {
			case proto.Message:
				accept := accepts(ProtobufContentTypes, r.Header["Accept"])
				if isProto(contentType(r)) || accept != "" {
					return writeProto(v, accept)
				} else {
					return writeJSON(v)
				}
			case string:
				http.Error(w, v, s.statusCode)
				s.Debugf("%s %s => %d: %s", r.Method, r.URL.Path, s.statusCode, v)
			case error:
				return v
			default:
				return writeJSON(v)
			}
		} else {
			s.Debugf("%s %s => %d", r.Method, r.URL.Path, s.statusCode)
		}
		return nil
	}()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.Debugf("%s %s => %d: %s", r.Method, r.URL.Path, http.StatusInternalServerError, err.Error())
	}
}

type Server struct {
	*log.Loggable
	jsonPrefix, jsonIndent string
	middlewares            []MiddlewareFunc
}

func NewServer(logger log.Logger) *Server {
	return &Server{
		Loggable:    log.NewLoggable(logger),
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

func isNil(v interface{}) bool {
	return v == nil || (reflect.TypeOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}

func notNil(v interface{}) bool {
	return !isNil(v)
}

func contentType(r *http.Request) string {
	return r.Header.Get(ContentType)
}

func isProto(mime string) bool {
	return isTypeOf(mime, ProtobufContentTypes)
}

func isTypeOf(mime string, types []string) bool {
	for _, t := range types {
		if strings.HasPrefix(mime, t) {
			return true
		}
	}
	return false
}

func accepts(types []string, accepts []string) string {
	for _, t := range types {
		for _, a := range accepts {
			if strings.HasPrefix(a, t) {
				return t
			}
		}
	}
	return ""
}
