package negative

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

func validateUser(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	return nil
}

// N6: Validation Error — non-database validation error message emitted to client.
func N6_ValidationErrorMessage(w http.ResponseWriter) {
	validationErr := validateUser("")
	if validationErr != nil {
		userFacingErr := validationErr.Error()
		http.Error(w, userFacingErr, http.StatusBadRequest)
	}
}

// N7: Local Buffer Write — writing error string to in-memory bytes.Buffer, not HTTP response.
func N7_BufferWrite(err error) {
	var buf bytes.Buffer
	_, _ = buf.Write([]byte(err.Error()))
}

// N8: Local Buffer Fprintf — formatting error into memory buffer via fmt.Fprintf.
func N8_BufferFprintf(err error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "internal log: %s", err.Error())
}

// N9: Local Buffer JSON Encode — encoding error into bytes.Buffer, not HTTP response.
func N9_BufferJsonEncode(err error) {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(err.Error())
}

// N10: JSON Decode Error — standard library decoder error emitted as client bad request.
func N10_JSONDecodeError(w http.ResponseWriter, r *http.Request) {
	var payload struct{ Name string }
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// N11: Static Constant 404 — static constant string emitted to 404 response.
func N11_StaticConstant_404(w http.ResponseWriter) {
	http.Error(w, "resource not found", http.StatusNotFound)
}

type FactoryOption func()

func WithCause(err error) FactoryOption { return func() {} }

func NewNotFound(code string, msg string, opts ...FactoryOption) {}

// N12: Masked Factory With Cause — static domain message with internal cause wrapping.
func N12_MaskedFactoryWithCause(err error) {
	NewNotFound("RESOURCE_NOT_FOUND", "Entity does not exist", WithCause(err))
}

// N13: Non-DB Error With DB Name — variable named sqlErr created via errors.New is non-database.
func N13_NonDBErrorWithDBName(w http.ResponseWriter) {
	sqlErr := errors.New("totally unrelated")
	http.Error(w, sqlErr.Error(), http.StatusBadRequest)
}

type CustomService struct{}

func (s *CustomService) Ping() error  { return errors.New("service unreachable") }
func (s *CustomService) Err() error   { return errors.New("custom error") }
func (s *CustomService) Close() error { return errors.New("failed closing client") }

// N14: Non-DB Service Methods — methods named Ping, Err, Close on custom non-database types.
func N14_NonDBServiceMethods(w http.ResponseWriter, s *CustomService) {
	if err := s.Ping(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
	if err := s.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
	if err := s.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

type MemoryStore struct{}

func (m *MemoryStore) Get(key string) (string, error) {
	return "", errors.New("key missing in cache")
}

// N15: Memory Store Error — receiver named store/repo on in-memory cache without database connection.
func N15_MemoryStoreError(w http.ResponseWriter, store *MemoryStore) {
	_, err := store.Get("item")
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}
}

