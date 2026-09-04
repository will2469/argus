package adversarial

import (
	"errors"
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

// A8: Non-DB Struct with DB Receiver Name — receiver named db with non-DB type Calculator must NOT be flagged.
type Calculator struct{}

func (Calculator) Exec(string) error { return nil }

func A8_CalculatorReceiverNamedDB(w http.ResponseWriter) {
	var db Calculator
	err := db.Exec("calculate")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// A9: Variable Shadowing — inner scope shadows DB error with clean errors.New, outer scope leaks DB error.
func A9_VariableShadowing(w http.ResponseWriter, pgErr *PgError) {
	err := pgErr
	{
		err := errors.New("sanitized client error")
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// A10: Branch Reassignment — error conditionally assigned from PgError (MAYBE_DB must be caught).
func A10_BranchReassignment(w http.ResponseWriter, pgErr *PgError, cond bool) {
	var err error = errors.New("initial")
	if cond {
		err = pgErr
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// A11: Custom Struct with Detail/Hint/Where — custom struct with same field names should NOT be flagged as PgError.
type CustomError struct {
	Detail string
	Hint   string
	Where  string
}

func (c *CustomError) Error() string {
	return c.Detail
}

func A11_CustomStructWithPgErrorFields(w http.ResponseWriter, customErr *CustomError) {
	// Access Detail field directly - should NOT be flagged since it's not pgconn.PgError
	_ = customErr.Detail
}

