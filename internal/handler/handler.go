package handler

import (
	"encoding/json"
	"net/http"

	"github.com/andressep95/aws-backup-bridge/signer-service/internal/service"
	"github.com/gorilla/mux"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	s3Service *service.S3Service
}

// NewHandler creates a new handler instance
func NewHandler(s3Service *service.S3Service) *Handler {
	return &Handler{
		s3Service: s3Service,
	}
}

// PresignedURLRequest represents the request body for presigned URL generation
type PresignedURLRequest struct {
	Filename    string            `json:"filename"`             // Just the filename, server will add inputs/date/time/ prefix
	ContentType string            `json:"content_type,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"` // Custom metadata headers (x-amz-meta-*)
}

// PresignedURLResponse represents the response for presigned URL
type PresignedURLResponse struct {
	URL       string `json:"url"`
	ExpiresIn string `json:"expires_in"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// SearchObject handles searching for a file by name
func (h *Handler) SearchObject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename string `json:"filename"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Filename == "" {
		respondWithError(w, http.StatusBadRequest, "filename is required", "")
		return
	}

	exists, objectKey, err := h.s3Service.SearchObjectByFilename(r.Context(), req.Filename)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to search object", err.Error())
		return
	}

	response := map[string]interface{}{
		"exists":   exists,
		"filename": req.Filename,
	}

	if exists {
		response["object_key"] = objectKey
	}

	respondWithJSON(w, http.StatusOK, response)
}

// GeneratePutURL handles PUT presigned URL generation for uploading
func (h *Handler) GeneratePutURL(w http.ResponseWriter, r *http.Request) {
	var req PresignedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Filename == "" {
		respondWithError(w, http.StatusBadRequest, "filename is required", "")
		return
	}

	url, fullPath, err := h.s3Service.GeneratePresignedPutURL(r.Context(), req.Filename, req.ContentType, req.Metadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate presigned URL", err.Error())
		return
	}

	// Log the generated path and URL for debugging
	println("Generated object path:", fullPath)
	println("Generated presigned URL query params (first 200 chars):", url[0:min(200, len(url))])

	respondWithJSON(w, http.StatusOK, PresignedURLResponse{
		URL:       url,
		ExpiresIn: "configured expiration time",
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "signer-service",
	})
}

// SetupRoutes configures all routes for the application
func (h *Handler) SetupRoutes() *mux.Router {
	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/health", h.HealthCheck).Methods("GET")

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/object/search", h.SearchObject).Methods("POST")
	api.HandleFunc("/presigned-url/upload", h.GeneratePutURL).Methods("POST")

	return router
}

// Helper functions

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Internal Server Error","message":"Failed to marshal response"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, error string, message string) {
	respondWithJSON(w, code, ErrorResponse{
		Error:   error,
		Message: message,
	})
}
