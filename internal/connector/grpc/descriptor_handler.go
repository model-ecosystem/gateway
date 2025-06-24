package grpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DescriptorHandler provides HTTP endpoints for descriptor management
type DescriptorHandler struct {
	manager *DescriptorManager
}

// NewDescriptorHandler creates a new descriptor handler
func NewDescriptorHandler(manager *DescriptorManager) *DescriptorHandler {
	return &DescriptorHandler{
		manager: manager,
	}
}

// ServeHTTP implements http.Handler
func (h *DescriptorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleList(w, r)
	case http.MethodPost:
		h.handleUpload(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleList lists all loaded descriptor files
func (h *DescriptorHandler) handleList(w http.ResponseWriter, r *http.Request) {
	files := h.manager.GetLoader().GetLoadedFiles()
	
	response := map[string]interface{}{
		"files": files,
		"count": len(files),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleUpload handles descriptor file upload
func (h *DescriptorHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get the file
	file, header, err := r.FormFile("descriptor")
	if err != nil {
		http.Error(w, "Failed to get file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file extension
	if !strings.HasSuffix(header.Filename, ".desc") {
		http.Error(w, "File must have .desc extension", http.StatusBadRequest)
		return
	}

	// Create temp file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "grpc-desc-"+header.Filename)

	// Save uploaded file
	dst, err := os.Create(tempFile)
	if err != nil {
		http.Error(w, "Failed to create temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Load the descriptor
	err = h.manager.AddDescriptorFile(tempFile)
	if err != nil {
		// Clean up temp file on error
		_ = os.Remove(tempFile)
		http.Error(w, "Failed to load descriptor: "+err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"message": "Descriptor loaded successfully",
		"file":    tempFile,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

// handleDelete removes a descriptor file from tracking
func (h *DescriptorHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	// Get file path from query parameter
	filePath := r.URL.Query().Get("file")
	if filePath == "" {
		http.Error(w, "Missing 'file' query parameter", http.StatusBadRequest)
		return
	}

	// Remove the descriptor file from config
	h.manager.RemoveDescriptorFile(filePath)

	response := map[string]interface{}{
		"message": "Descriptor removed from configuration (restart required)",
		"file":    filePath,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// RegisterRoutes registers the descriptor management routes
func (h *DescriptorHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/grpc/descriptors", h)
	mux.HandleFunc("/api/grpc/descriptors/reload", h.handleReload)
}

// handleReload forces a reload of all descriptors
func (h *DescriptorHandler) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get specific file from query parameter (optional)
	filePath := r.URL.Query().Get("file")

	var err error
	var message string

	if filePath != "" {
		// Reload specific file
		loader := h.manager.GetLoader()
		err = loader.ReloadFile(filePath)
		message = fmt.Sprintf("Reloaded descriptor file: %s", filePath)
	} else {
		// Reload all files
		// This is a bit hacky - we're calling the private reload method
		// In a real implementation, we'd expose a public Reload method
		files := h.manager.GetLoader().GetLoadedFiles()
		for _, file := range files {
			if reloadErr := h.manager.GetLoader().ReloadFile(file); reloadErr != nil {
				err = reloadErr
				break
			}
		}
		message = "Reloaded all descriptor files"
	}

	if err != nil {
		http.Error(w, "Failed to reload: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message": message,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}