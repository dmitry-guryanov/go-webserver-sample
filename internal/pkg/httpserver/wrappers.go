package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"

	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/log"
)

type requestKey int

const maxBodyLen = 1024 * 1024

func toJSON(src interface{}) []byte {
	bytes, err := json.Marshal(src)
	if err != nil {
		panic(fmt.Sprintf("Failed to encode to JSON, input: %v", src))
	}

	return bytes
}

func parseJSONBody(r *http.Request, obj interface{}) error {
	if r.Body == nil {
		return httpStatusString(http.StatusBadRequest, "Non-empty request body is expected.")
	}
	lr := io.LimitReader(r.Body, maxBodyLen)

	d := json.NewDecoder(lr)
	d.DisallowUnknownFields()
	if err := d.Decode(obj); err != nil {
		return httpStatusError(http.StatusBadRequest, errors.Wrap(err, "reading/parsing JSON HTTP Body"))
	}

	if d.More() {
		return httpStatusString(http.StatusBadRequest, "HTTP Body contains extra data after JSON object")
	}

	return nil
}

// HTTPStatusError allows to return errors which can be
// converted to proper HTTP status codes.
type HTTPStatusError interface {
	error
	HTTPStatusCode() int
}

type statusErrorImpl struct {
	code int
	err  error
}

func (se *statusErrorImpl) Error() string {
	return se.err.Error()
}

func (se *statusErrorImpl) HTTPStatusCode() int {
	return se.code
}

func httpStatusError(code int, err error) error {
	if err == nil {
		err = fmt.Errorf(http.StatusText(code))
	}

	return &statusErrorImpl{
		code: code,
		err:  err,
	}
}

func httpStatusString(code int, msg string) error {
	return &statusErrorImpl{
		code: code,
		err:  fmt.Errorf(msg),
	}
}

func httpStatus(code int) error {
	return httpStatusError(code, nil)
}

// jsonHandleFunc is special type of HTTP handler which returns object and an error.
// When wrapped with newJsonHandler it writes the object to the HTTP connection in
// JSON format if it's not nil. If error implements HTTPStatusError interface,
// status is taken from the error. If status is an error, not 2xx, then instead of
// returned object (which should be nil) wrapper writes error message to the HTTP
// connection.
type jsonHandleFunc func(w http.ResponseWriter, r *http.Request) (interface{}, error)
type jsonHandler struct {
	f jsonHandleFunc
}

func isBadCode(code int) bool {
	return code/100 != 2
}

func (h *jsonHandler) serve(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	obj, err := h.f(w, r)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, nil
	}

	return json.Marshal(obj)
}

func newJSONHandler(f jsonHandleFunc) http.Handler {
	h := jsonHandler{
		f: f,
	}

	return newDataHandler(h.serve)
}

type dataHandleFunc func(w http.ResponseWriter, r *http.Request) ([]byte, error)
type dataHandler struct {
	f dataHandleFunc
}

func (h *dataHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, err := h.f(w, r)

	cause := errors.Cause(err)

	code := http.StatusInternalServerError
	httpErr, ok := cause.(HTTPStatusError)
	if ok {
		code = httpErr.HTTPStatusCode()
	} else if cause == nil {
		code = http.StatusOK
	}

	if code == http.StatusNoContent {
		data = nil
	}

	if code == http.StatusInternalServerError {
		log.L(r.Context()).WithError(err).Error("Error handling request")
	}

	if isBadCode(code) {
		obj := JSONErrorStruct{
			Code: code,
			Msg:  cause.Error(),
		}
		data = toJSON(obj)
		log.L(r.Context()).Infof("Request completed with an error: %s", obj.Msg)
	}

	if data != nil {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(code)

	if _, err := w.Write(data); err != nil {
		log.L(r.Context()).WithError(err).Error("failed to write data to HTTP connection")
	}
}

func newDataHandler(f dataHandleFunc) http.Handler {
	return &dataHandler{f}
}
