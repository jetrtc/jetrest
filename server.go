package jetrest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/jetrtc/jetlog"
)

const (
	ContentType     = "Content-Type"
	JsonContentType = "application/json"
)

var (
	ProtobufContentTypes = []string{"application/protobuf", "application/x-protobuf"}
)

type Session struct {
	Status         int
	Context        context.Context
	Request        *http.Request
	ResponseWriter http.ResponseWriter
	jetlog.Loggable
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
		Status:         http.StatusOK,
		Context:        r.Context(),
		Request:        r,
		ResponseWriter: w,
		Loggable:       rt.server.Loggable,
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
		w.WriteHeader(s.Status)
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
		s.Debugf("%s %s => %d: %s", r.Method, r.URL.Path, s.Status, accept)
		return nil
	}
	writeJSON := func(v interface{}) error {
		w.Header().Set(ContentType, JsonContentType)
		w.WriteHeader(s.Status)
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
		s.Debugf("%s %s => %d: %s", r.Method, r.URL.Path, s.Status, JsonContentType)
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
				http.Error(w, v, s.Status)
				s.Debugf("%s %s => %d: %s", r.Method, r.URL.Path, s.Status, v)
			case error:
				return v
			default:
				return writeJSON(v)
			}
		} else {
			if s.Status != http.StatusOK {
				w.WriteHeader(s.Status)
			}
			s.Debugf("%s %s => %d", r.Method, r.URL.Path, s.Status)
		}
		return nil
	}()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.Debugf("%s %s => %d: %s", r.Method, r.URL.Path, http.StatusInternalServerError, err.Error())
	}
}

type Server struct {
	jetlog.Loggable
	jsonPrefix, jsonIndent string
	middlewares            []MiddlewareFunc
}

func NewServer(logger jetlog.Logger) *Server {
	return &Server{
		Loggable: jetlog.Loggable{
			Logger: logger,
		},
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
