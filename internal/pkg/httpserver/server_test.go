package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/log"
	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/wsnotify"
)

var l = log.L(context.TODO())
var srv *HTTPServer

func setLocal(req *http.Request) {
	req.RemoteAddr = "127.0.0.1:123"
}

func setRemote(req *http.Request) {
	req.RemoteAddr = "8.8.8.8:111"
}

func mustUnmarshal(data []byte, v interface{}) {
	if err := json.Unmarshal(data, v); err != nil {
		panic(fmt.Sprintf("Error json unmarshal: %v", err))
	}
}

func TestMain(m *testing.M) {
	hub := wsnotify.NewHub()
	go hub.Run()

	srv = initHandlers(hub)

	ret := m.Run()

	os.Exit(ret)
}

func newRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, url, body)
	require.NoError(t, err, "Error initializing http.Request")

	return req
}

func runRequestHandler(t *testing.T, h http.Handler, req *http.Request, status int) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, status, rr.Code, "handler returned wrong status code")

	if status != http.StatusOK {
		/* Validate also body */
		var msg JSONErrorStruct

		d := json.NewDecoder(rr.Body)
		d.DisallowUnknownFields()

		require.NoError(t, d.Decode(&msg), "Invalid error message")
		require.False(t, d.More(), "Extra data after error-msg JSON, possibly no return after error")
		require.Equal(t, status, msg.Code, "Invalid 'Code' in error message")
		require.True(t, len(msg.Msg) > 3, "Error message is too short")
	}

	return rr
}

func runRequest(t *testing.T, req *http.Request, status int) *httptest.ResponseRecorder {
	return runRequestHandler(t, srv.handler, req, status)
}

func TestSimple(t *testing.T) {
	req := newRequest(t, "GET", "/simple", nil)
	rr := runRequest(t, req, http.StatusOK)

	require.Equal(t, []byte("Test"), rr.Body.Bytes())
}

func TestJSON(t *testing.T) {
	req := newRequest(t, "GET", "/json", nil)
	rr := runRequest(t, req, http.StatusOK)

	var s someStruct
	mustUnmarshal(rr.Body.Bytes(), &s)

	require.Equal(t, s.Value, 15)

}
func TestData(t *testing.T) {
	req := newRequest(t, "GET", "/data", nil)
	rr := runRequest(t, req, http.StatusOK)

	require.Equal(t, []byte("123"), rr.Body.Bytes())
}

func TestError400(t *testing.T) {
	req := newRequest(t, "GET", "/error400", nil)
	runRequest(t, req, http.StatusBadRequest)
}
