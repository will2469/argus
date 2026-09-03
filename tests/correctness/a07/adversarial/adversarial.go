package adversarial

import (
	"net/http"
)

type PgError struct {
	Code    string
	Message string
	Detail  string
	Hint    string
	Where   string
}

func (p *PgError) Error() string {
	return p.Message
}

// A1: Branch — conditional error leakage in debug/dev branch.
func A1_Branch(w http.ResponseWriter, err error, isDev bool) {
	if isDev {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// A2: Reassignment — safe error message reassigned to raw err.Error().
func A2_Reassignment(w http.ResponseWriter, err error) {
	msg := "internal error"
	_ = msg
	msg = err.Error()
	http.Error(w, msg, http.StatusInternalServerError)
}

// A3: Alias — error aliased through variable indirection.
func A3_Alias(w http.ResponseWriter, err error) {
	dbErr := err
	http.Error(w, dbErr.Error(), http.StatusInternalServerError)
}

// A4: Wrapper — response writer struct method leaking error.
type APIWriter struct {
	w http.ResponseWriter
}

func (a *APIWriter) WriteErr(err error) {
	http.Error(a.w, err.Error(), http.StatusInternalServerError)
}

// A5: Nested Function — closure capturing response writer and leaking error.
func A5_NestedFunction(w http.ResponseWriter, err error) {
	writeResp := func() {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	writeResp()
}

// A6: Generic — generic responder struct leaking error.
type Responder[T any] struct {
	w http.ResponseWriter
}

func (r *Responder[T]) Fail(err error) {
	http.Error(r.w, err.Error(), http.StatusInternalServerError)
}

// A7: Interface — direct access to sensitive PgError.Hint field.
func A7_SensitiveField(pgErr *PgError) string {
	return pgErr.Hint
}
