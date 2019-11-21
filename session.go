package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
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
	log.Context
	server         *Server
	Data           map[interface{}]interface{}
	Request        *http.Request
	ResponseWriter http.ResponseWriter
}

func (s *Session) RemoteAddr() net.IP {
	addr := s.Request.RemoteAddr
	fwds := s.Request.Header["X-Forwarded-For"]
	if fwds != nil {
		fwd := fwds[0]
		splits := strings.Split(fwd, ",")
		addr = splits[0]
	}
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		addr = host
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		s.Errorf("Failed to parse remote addr: %s => %s", addr, err.Error())
	}
	return ip
}

func (s *Session) RequestHeader() http.Header {
	return s.Request.Header
}

func (s *Session) ResponseHeader() http.Header {
	return s.ResponseWriter.Header()
}

func (s *Session) Decode(val interface{}) error {
	switch v := val.(type) {
	case proto.Message:
		if isProto(contentType(s.Request)) {
			data, err := ioutil.ReadAll(s.Request.Body)
			if err != nil {
				s.Errorf("Failed to read request body: %s", err.Error())
				return err
			}
			err = proto.Unmarshal(data, v)
			if err != nil {
				s.Errorf("Failed to unmarshal proto request body: %s", err.Error())
			}
			return err
		} else {
			err := json.NewDecoder(s.Request.Body).Decode(v)
			if err != nil {
				s.Errorf("Failed to decode JSON request body: %s", err.Error())
				return err
			}
			return nil
		}
	default:
		return json.NewDecoder(s.Request.Body).Decode(v)
	}
}

func (s *Session) Encode(val interface{}) error {
	switch v := val.(type) {
	case proto.Message:
		accept := accepts(ProtobufContentTypes, s.RequestHeader()["Accept"])
		if isProto(contentType(s.Request)) || accept != "" {
			return s.encodeProto(v, accept)
		} else {
			return s.encodeJSON(v)
		}
	default:
		return s.encodeJSON(v)
	}
}

func (s *Session) Status(code int) {
	s.Statusf(code, http.StatusText(code))
}

func (s *Session) Statusf(code int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	s.Debugf("Writing status: %d \"%s\"", code, msg)
	http.Error(s.ResponseWriter, msg, code)
}

func (s *Session) Error(err error) {
	s.Statusf(500, err.Error())
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

func (s *Session) encodeProto(v proto.Message, accept string) error {
	if accept == "" {
		accept = ProtobufContentTypes[0]
	}
	s.ResponseHeader().Set(ContentType, accept)
	data, err := proto.Marshal(v)
	if err != nil {
		s.Errorf("Failed to encode protobuf: %s", err.Error())
		return err
	}
	s.Debugf("Writing protobuf: %d bytes", len(data))
	_, err = io.Copy(s.ResponseWriter, bytes.NewBuffer(data))
	if err != nil {
		s.Errorf("Failed to write protobuf: %s", err.Error())
		return err
	}
	return nil
}

func (s *Session) encodeJSON(v interface{}) error {
	s.ResponseHeader().Set(ContentType, JsonContentType)
	data, err := json.MarshalIndent(v, s.server.jsonPrefix, s.server.jsonIndent)
	if err != nil {
		s.Errorf("Failed to encode JSON: %s", err.Error())
		return err
	}
	s.Debugf("Writing JSON: %d bytes", len(data))
	_, err = s.ResponseWriter.Write(data)
	if err != nil {
		s.Errorf("Failed to write JSON: %s", err.Error())
		return err
	}
	return nil
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
