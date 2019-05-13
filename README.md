# jetrest

This package leverages the power of ```gorilla/mux``` and ```protobuf``` to provide a REST framework with the following characteristics:

* JSON/```protobuf``` dual formats support.
* Automatic validation against required/optional properties.

Let's start with a simple REST user service:

```go
func main() {
    r := mux.NewRouter()
    rest := jetrest.NewServer()
    r.Path("/user/{id:[a-z]+}").Handler(rest.HandlerFunc(UserHandler))
    log.Fatal(http.ListenAndServe("localhost:8080", r))
}
```

And then implement a handler function:

```go
func UserHandler(s *jetrest.Session) interface{} {
    // handle session and return either an error, or a response, or nil
}
```

Either of the following could be returned in a handler function:

* ```nil```: Status ```200 OK``` will be replied with nothing written to the ```ResponseWriter```.
* ```error```: Status code ```500``` will be replied with message of ```err.Error()```, and nothing written to the ```ResponseWriter```.
* A special ```error``` created by ```jrest.HTTPError(code int)``` or  ```jrest.HTTPErrorf(code int, format string, ...)```: custom status code with be replied, with message created by ```http.StatusText(code int)``` or custom message.
* Otherwise, status ```200 OK``` will be replied, and the returned object will be encoded and written to the ```ResponseWriter```. By default it will be encoded in JSON, unless both server and client support ```protobuf```. However, if there was error encoding the response, status code ```500``` will be replied instead. So be sure to set required properties of data object in ```protobuf``` format. How the dual format works will be described later.

Before finishing the handler function, define the data structure of ```User``` and a simple database with ```map```.

```go
type User struct {
    Email       string `json:"email"`
    DisplayName string `json:"display_name,omitempty"`
}
var users = map[string]*User{
    "alice": &User{"alice@foo.com", "Alice"},
}
```

And then we can finish the handler function.

```go
func UserHandler(s *jetrest.Session) interface{} {
    id := s.Var("id")
    switch s.Request.Method {
    case "GET":
        return users[id]
    case "POST":
        user := &User{}
        err := s.Decode(user)
        if err != nil {
            return err
        }
        users[id] = user
    case "DELETE":
        delete(users, id)
    }
    return nil
}
```

Now we have a REST user service with just few lines of code, but only JSON format was supported. So this doesn't bring obvious benefit compared will ```gorilla/mux```. So let's move on.

## How Protobuf Help JSON Validation

In the past, JSON validation is tending to be tidious and error prone. Assume that we expected a ```User``` should has a mandatory ```email``` property and an optional ```display_name``` property.

However, all the following JSON data could be decoded by ```json.Unmarshal()``` without returning ```error```:

```json
{"email":"alice@foo.com","name":"Alice"}
```

```json
{"email":"alice@foo.com"}
```

```json
{"name":"Alice"}
```

```json
{}
```

And all decoded ```User``` can also be encoded by ```json.Marshal()``` without returning ```error```. But the corresponding output JSON data become:

```json
{"email":"alice@foo.com","name":"Alice"}
```

```json
{"email":"alice@foo.com"}
```

```json
{"email":"","name":"Alice"}
```

```json
{"email":""}
```

It's obvious and well-known that ```encoding/json``` does nothing about data validation. Struct tag ```omitempty``` only help marshal empty properties. So data validation was left on us by checking properties one-by-one:

```go
user := &User{}
err := json.Unmarshal(data, user)
if err != nil {
    return err
}
if user.Email == "" {
    return err.Errorf("Empty e-mail")
}
// do actual stuffs
```

However, if we define ```User``` in ```.proto``` and generate its golang code:

```protobuf
message User {
    required string email = 1;
    optional string display_name = 2;
}
```

We can leverage ```protobuf``` to help basic data validation against required/optional:

```go
user := &User{}
err := json.Unmarshal(data, user)
if err == nil {
    _, err = proto.Marshal(user)
}
if err != nil {
    return err
}
// do actual stuffs
```

If the argument passed to ```Session.Decode()``` is an instance of ```proto.Message```, it will do all the data validation magic. No matter the actual request was in JSON or protobuf request. As a result, it's encouraged to define data objects in ```.proto``` if the code generation overhead is acceptable.

However, required/optional was not supported by ```proto3``` but only by ```proto2```. Even so, ```protobuf``` still have the following benefits:

* Help convert snake-case naming convention to camel-case, saving us from defining tons of JSON struct tags.
* Help marshal/unmarshal for both server side and client side.
* More data efficient compared with JSON.

## How Dual Format was Supported

If all the following criteria were met, request body will be decoded as a ```protobuf``` message:

* The argument passed to ```Session.Decode()``` is an instance of ```proto.Message```.
* The value of request header ```Content-Type``` matches ```application/protobuf``` or ```application/x-protobuf```.

Otherwise, the request body will be decoded as JSON message.

If all the following criteria were met, the response will be encoded as a ```protobuf``` message:

* The returned object is an instance of ```proto.Message```.
* The request was decoded as ```protobuf``` message, or values of request header ```Accept``` matches ```application/protobuf``` or ```application/x-protobuf```.

With dual format support, a REST service can keep backward compatibility and interoperability with ```javascript``` clients, and at the same time embrace the data security and efficient of ```protobuf``` format.

## Alternatives

* ```grpc``` by Google: Also based on ```protobuf```, but ```grpc``` is not RESTful. However, it's possible to keep backward compatibility by providing a REST broker in front of ```grpc``` service. In ```grpc``` both data object and service could be defined by ```.proto```. And corresponding server stubs and client could be code-generated in many supported programming languages.
* ```swagger```: Swagger is a framework for generating REST services as well as its documents based on ```OpenAPI```. However, AFAIK it doesn't support ```protobuf``` over REST. And in [some article](https://auth0.com/blog/beating-json-performance-with-protobuf/), you can learn that ```protobuf``` is much more efficient than JSON.

Besides, there is an interesting project ```grpc-gateway```, which could be integrated into ```protoc``` code-gen process to generate a gateway in front of ```grpc```, and a generated REST service conforming to ```swagger```.

However IMHO, REST is resource-oriented, but RPC is operation-oriented. They are of different paradigms. The code generation approach result in either a strange ```grpc``` service or a REST service which is not backward-compatible.

## Sample Code

Sample code of the REST user service, powered by ```protobuf```, was replaced under ```/sample``` folder. Before running the sample, you would have to code-gen from ```user.proto``` first.

```shell
protoc --go_out=. user.proto
```

To run the REST user service:

```shell
go run user.go user.pb.go
```

To play with curl, you can try requests like this:

```shell
curl http://127.0.0.1:8080/user/alice
```

And you can also take a look at ```user_test.go``` or just run it:

```shell
go test
```
