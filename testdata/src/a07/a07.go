package a07

import (
	"encoding/json"
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

type ResponseHelper struct{}

func (ResponseHelper) ErrorJSON(w http.ResponseWriter, code int, msg string) {}

var response ResponseHelper

func SafeConstantMessage(w http.ResponseWriter, err error) {
	http.Error(w, "internal server error occurred", http.StatusInternalServerError)
}

func SafeSqlStateBranching(w http.ResponseWriter, err error) {
	var pgErr *PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			response.ErrorJSON(w, http.StatusConflict, "resource already exists")
			return
		}
	}
	response.ErrorJSON(w, http.StatusInternalServerError, "failed processing request")
}

func BadHttpError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError) // want `\[ARGUS-A07\] raw err\.Error\(\) passed directly to HTTP response`
}

func BadPgErrorDetail(w http.ResponseWriter, pgErr *PgError) {
	response.ErrorJSON(w, http.StatusBadRequest, pgErr.Detail) // want `\[ARGUS-A07\] forbidden direct access to pgconn\.PgError\.Detail`
}

func BadDirectAccess(pgErr *PgError) string {
	return pgErr.Detail // want `\[ARGUS-A07\] forbidden direct access to pgconn\.PgError\.Detail`
}

func BadIndirectVariable(w http.ResponseWriter, err error) {
	errStr := err.Error()
	http.Error(w, errStr, http.StatusInternalServerError) // want `\[ARGUS-A07\] variable "errStr" derived from raw err\.Error\(\) passed to HTTP response`
}

func BadRawWrite(w http.ResponseWriter, err error) {
	w.Write([]byte(err.Error())) // want `\[ARGUS-A07\] raw err\.Error\(\) passed directly to HTTP response`
}

func BadJsonEncode(w http.ResponseWriter, err error) {
	json.NewEncoder(w).Encode(err.Error()) // want `\[ARGUS-A07\] raw err\.Error\(\) passed directly to HTTP response`
}

func IgnoredErrorLeak(w http.ResponseWriter, err error) {
	// argus:ignore ARGUS-A07 internal debug diagnostic output
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
