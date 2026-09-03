package negative

import (
	"encoding/json"
	"errors"
	"log"
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

// N1: Obvious Safe — generic constant static error message emitted to client.
func N1_ObviousSafe(w http.ResponseWriter) {
	http.Error(w, "internal server error occurred", http.StatusInternalServerError)
}

// N2: Legitimate Idiom — checking SQLSTATE error code and mapping to user-safe constant response.
func N2_LegitimateIdiom(w http.ResponseWriter, err error) {
	var pgErr *PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			response.ErrorJSON(w, http.StatusConflict, "resource already exists")
			return
		}
	}
	response.ErrorJSON(w, http.StatusInternalServerError, "failed processing request")
}

// N3: Unrelated API — error logged to server-side stdout/logger, not emitted to response sink.
func N3_UnrelatedAPI(err error) {
	log.Printf("internal database connection error: %v", err.Error())
}

// N4: Sanitized Input — validation error with sanitized client message.
func N4_SanitizedInput(w http.ResponseWriter) {
	http.Error(w, "invalid request parameter: page must be positive integer", http.StatusBadRequest)
}

// N5: Static Constant Input — static constant struct encoded into JSON response.
type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

func N5_StaticConstant(w http.ResponseWriter) {
	res := ErrorResponse{
		Message: "entity not found",
		Code:    "ERR_NOT_FOUND",
	}
	_ = json.NewEncoder(w).Encode(res)
}
