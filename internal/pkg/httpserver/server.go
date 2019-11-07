package httpserver

import (
	"fmt"
	"net"
	"net/http"

	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/log"
	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/wsnotify"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// JSONErrorStruct is a struct to be marshaled to JSON and
// reporting to the HTTP client.
type JSONErrorStruct struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
}

type HTTPServer struct {
	listener net.Listener
	handler  http.Handler
	hub      *wsnotify.Hub
}

func (srv *HTTPServer) handleSimple(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Test"))
}

type someStruct struct {
	Value int
}

func (srv *HTTPServer) handleJSON(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return someStruct{15}, nil
}

func (srv *HTTPServer) handleData(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	return []byte("123"), nil
}

func (srv *HTTPServer) handleError400(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return nil, httpStatus(http.StatusBadRequest)
}

func (srv *HTTPServer) serveWs(w http.ResponseWriter, r *http.Request) {
	log.L(r.Context()).Info("Start service websocket")
	if err := srv.hub.ServeWs(w, r, 745); err != nil {
		code := http.StatusInternalServerError
		http.Error(w, http.StatusText(code), code)
	}
}

func initHandlers(hub *wsnotify.Hub) *HTTPServer {
	srv := &HTTPServer{
		hub: hub,
	}

	globalHandlers := []handlerFactoryFunc{
		setLogHandler,
		setReqIDHandler,
	}

	r := mux.NewRouter()
	r.HandleFunc("/simple", srv.handleSimple)
	r.Handle("/json", newJSONHandler(srv.handleJSON))
	r.Handle("/data", newDataHandler(srv.handleData))
	r.Handle("/error400", newJSONHandler(srv.handleError400))
	r.HandleFunc("/ws", srv.serveWs)

	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Origin", "Options", "Accept", "Content-Type", "Cache-Control", "Content-Language", "Expires", "Last-Modified", "Pragma", "X-Session-Id", "Authorization"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE", "PATCH"})
	exposedHeaders := handlers.ExposedHeaders([]string{"X-Auth-Token"})

	srv.handler = handlers.CORS(originsOk, headersOk, methodsOk, exposedHeaders)(registerHandlers(r, globalHandlers...))
	return srv
}

func New(hub *wsnotify.Hub, port int) (*HTTPServer, error) {
	srv := initHandlers(hub)

	http.Handle("/", srv.handler)

	var err error
	if srv.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port)); err != nil {
		return nil, err
	}

	return srv, nil
}

// Run initializes and starts HTTP server
func (srv *HTTPServer) Run() error {
	return http.Serve(srv.listener, nil)
}

// Close stops listening and deinitializes HTTP server
func (srv *HTTPServer) Close() error {
	return srv.listener.Close()
}
