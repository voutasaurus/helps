package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	a := &api{
		mux: http.NewServeMux(),
		log: log.New(os.Stdout, "helps: ", log.Llongfile|log.LstdFlags|log.LUTC),
	}

	routes := map[string]http.HandlerFunc{
		"/":        a.defaultHandler,
		"/healthz": a.healthzHandler,
		"/example": a.exampleHandler,
	}
	for p, h := range routes {
		a.mux.HandleFunc(p, h)
	}

	a.log.Println("starting server on :9090")
	a.log.Fatal(http.ListenAndServe(":9090", a.mux))
}

var (
	errBroken = errors.New("server broken")
)

type api struct {
	mux *http.ServeMux
	log *log.Logger
}

func (a *api) defaultHandler(w http.ResponseWriter, r *http.Request) {}
func (a *api) healthzHandler(w http.ResponseWriter, r *http.Request) {}

func (a *api) exampleHandler(w http.ResponseWriter, r *http.Request) {
	a.error(w, error500(fmt.Errorf("exampleHandler: %v", errBroken)))
}

/*
Examples:
	a.error(w, error500(err))

	a.error(w, error400(err, "bad json"))

	a.error(w, error400(err, "id empty"))

	a.error(w, error404(err, "entry %q not found", entryid))
*/

// error logs an error to the client and the server logs, linking the external
// facing error and the internal error via a random unique ID. It relies on
// httpError's contract for preventing external visibility of internal error
// details.
func (a *api) error(w http.ResponseWriter, err *httpError) {
	id, genErr := genUUID()
	if genErr != nil {
		a.log.Printf("genErr=%v, msg=%q", genErr, "error while reporting API error")
		// continue since we can still report the error with an empty errID
	}
	w.Header().Set("X-Errid", id)
	w.WriteHeader(err.code)
	encodeErr := json.NewEncoder(w).Encode(err)
	if encodeErr != nil {
		a.log.Printf("errID=%q, err=%v, encodeErr=%v, msg=%v", id, err, encodeErr, "error while reporting API error")
		return
	}
	a.log.Printf("errID=%q, err=%v", id, err)
}

// genUUID generates a random hex string in the UUID format:
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func genUUID() (string, error) {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		return "", fmt.Errorf("genUUID: %v", err)
	}

	buf := make([]byte, 36)

	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], u[10:])

	return string(buf), nil
}

// httpError is an error with dual internal / external use.
// It fmt's to the underlying error for recording internal errors.
// It json.Marshal's to the msg field with optional args in fmt format for
// external use.
type httpError struct {
	error
	code int
	msg  string
	args []interface{}
}

func newHTTPError(err error, code int, msg string, args ...interface{}) *httpError {
	return &httpError{
		error: err,
		code:  code,
		msg:   msg,
		args:  args,
	}
}

func error500(err error) *httpError {
	return newHTTPError(err, 500, "internal server error")
}

func error400(err error, msg string, args ...interface{}) *httpError {
	return newHTTPError(err, 400, msg, args...)
}

func error404(err error, msg string, args ...interface{}) *httpError {
	return newHTTPError(err, 404, msg, args...)
}

func (err *httpError) MarshalJSON() ([]byte, error) {
	msg := struct {
		Err string `json:"err"`
	}{
		Err: fmt.Sprintf(err.msg, err.args...),
	}
	return json.Marshal(msg)
}
