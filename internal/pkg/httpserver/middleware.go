package httpserver

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/log"
	"github.com/sirupsen/logrus"
)

type handlerFactoryFunc func(http.Handler) http.Handler
type contextKey int

const (
	_ contextKey = iota
	reqIDContextKey
)

var reqIDSeq uint64
var reqIDSeqLock sync.Mutex

type reqIDHandler struct {
	handler http.Handler
}

func (h *reqIDHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqIDSeqLock.Lock()
	r = r.WithContext(context.WithValue(r.Context(), reqIDContextKey, reqIDSeq))
	reqIDSeq++
	reqIDSeqLock.Unlock()

	h.handler.ServeHTTP(w, r)
}

// SetReqIDHandler attaches middleware which adds unique ID to
// HTTP requests.
func setReqIDHandler(handler http.Handler) http.Handler {
	return &reqIDHandler{handler}
}

type logHandler struct {
	handler http.Handler
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}

	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hw, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		w.WriteHeader(500)
		return nil, nil, fmt.Errorf("ResponseWriter doesn't imprelent http.Hijacker interface")
	}

	return hw.Hijack()
}

func (h *logHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := r.Context().Value(reqIDContextKey).(uint64)

	rlog := log.L(r.Context()).WithFields(logrus.Fields{
		"src":         "api",
		"req_id":      reqID,
		"method":      r.Method,
		"URL":         r.URL,
		"remote_addr": r.RemoteAddr,
	})
	r = r.WithContext(log.WithLogger(r.Context(), rlog))

	log.L(r.Context()).Infof("Start handling request")
	log.L(r.Context()).WithField("headers", r.Header).Debugf("Headers")
	sw := statusWriter{ResponseWriter: w}
	h.handler.ServeHTTP(&sw, r)
	log.L(r.Context()).WithField("status", sw.status).Infof("Finish handling request")
}

// SetLogHandler attaches logging middleware to the handler, which
// writes messages before and after processing requests
func setLogHandler(handler http.Handler) http.Handler {
	return &logHandler{handler}
}

// RegisterHandlers attaches given middleware to the http  handler.
func registerHandlers(h http.Handler, factories ...handlerFactoryFunc) http.Handler {
	for _, hf := range factories {
		h = hf(h)
	}

	return h
}
