package positive

import (
	"encoding/json"
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

type ResponseHelper struct{}

func (ResponseHelper) ErrorJSON(w http.ResponseWriter, code int, msg string) {}

var response ResponseHelper

// P1: Obvious Violation — raw err.Error() passed directly to http.Error sink.
func P1_Obvious(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError) // want `\[ARGUS-A07\] raw err\.Error\(\) passed directly to HTTP response`
}

// P2: Indirect Violation — variable derived from err.Error() passed to response sink.
func P2_Indirect(w http.ResponseWriter, err error) {
	errStr := err.Error()
	http.Error(w, errStr, http.StatusInternalServerError) // want `\[ARGUS-A07\] variable "errStr" derived from raw err\.Error\(\) passed to HTTP response`
}

// P3: Helper Violation — sensitive pgErr.Detail passed to response helper.
func P3_Helper(w http.ResponseWriter, pgErr *PgError) {
	response.ErrorJSON(w, http.StatusBadRequest, pgErr.Detail) // want `\[ARGUS-A07\] forbidden direct access to pgconn\.PgError\.Detail`
}

// P4: Nested Violation — direct extraction of sensitive pgErr.Detail in return.
func P4_Nested(pgErr *PgError) string {
	return pgErr.Detail // want `\[ARGUS-A07\] forbidden direct access to pgconn\.PgError\.Detail`
}

// P5: Alias Violation — raw err.Error() passed via direct w.Write or json.Encode.
func P5_Alias(w http.ResponseWriter, err error) {
	_, _ = w.Write([]byte(err.Error()))        // want `\[ARGUS-A07\] raw err\.Error\(\) passed directly to HTTP response`
	_ = json.NewEncoder(w).Encode(err.Error()) // want `\[ARGUS-A07\] raw err\.Error\(\) passed directly to HTTP response`
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(w http.ResponseWriter, err error) {
	// argus:ignore ARGUS-A07 internal debug diagnostic output
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
