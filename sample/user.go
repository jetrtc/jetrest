package main

import (
	"log"
	"net/http"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/jetrtc/jetlog"
	"github.com/jetrtc/jetrest"
)

func main() {
	log.Fatal(http.ListenAndServe("localhost:8080", newHandler()))
}

func newHandler() http.Handler {
	r := mux.NewRouter()
	rest := jetrest.NewServer(jetlog.NewDefaultLogger(log.New(os.Stderr, "", log.LstdFlags)))
	r.Path("/user/{id:[a-z]+}").Handler(rest.HandlerFunc(UserHandler))
	return r
}

var users = map[string]*User{
	"alice": &User{
		Email:       proto.String("alice@foo.com"),
		DisplayName: proto.String("Alice"),
	},
}

func UserHandler(s *jetrest.Session) interface{} {
	id := s.Var("id", "")
	switch s.Request.Method {
	case "GET":
		user := users[id]
		if user == nil {
			s.Status = 404
			return nil
		}
		return user
	case "POST":
		user := &User{}
		err := s.Decode(user)
		if err != nil {
			s.Status = 400
			return nil
		}
		users[id] = user
	case "DELETE":
		if users[id] == nil {
			s.Status = 404
			return nil
		}
		delete(users, id)
	default:
		s.Status = 405
		return nil
	}
	return nil
}
