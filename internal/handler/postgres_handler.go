package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/mrckurz/CI-CD-MCM/internal/model"
	"github.com/mrckurz/CI-CD-MCM/internal/store"
)

// PostgresHandler holds the dependencies for PostgreSQL-backed HTTP handlers.
type PostgresHandler struct {
	Store *store.PostgresStore
}

// NewPostgresHandler creates a new PostgresHandler.
func NewPostgresHandler(s *store.PostgresStore) *PostgresHandler {
	return &PostgresHandler{Store: s}
}

// RegisterRoutes sets up the API routes.
func (h *PostgresHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", h.Health).Methods("GET")
	r.HandleFunc("/products", h.GetProducts).Methods("GET")
	r.HandleFunc("/products", h.CreateProduct).Methods("POST")
	r.HandleFunc("/products/{id:[0-9]+}", h.GetProduct).Methods("GET")
	r.HandleFunc("/products/{id:[0-9]+}", h.UpdateProduct).Methods("PUT")
	r.HandleFunc("/products/{id:[0-9]+}", h.DeleteProduct).Methods("DELETE")
}

func (h *PostgresHandler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.DB.Ping(); err != nil {
		respondError(w, http.StatusServiceUnavailable, "database unreachable")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *PostgresHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.Store.GetAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, products)
}

func (h *PostgresHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	p, err := h.Store.GetByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Product not found")
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (h *PostgresHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var p model.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if !p.Validate() {
		respondError(w, http.StatusBadRequest, "Invalid product: name required, price must be >= 0")
		return
	}

	created, err := h.Store.Create(p)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

func (h *PostgresHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	var p model.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	updated, err := h.Store.Update(id, p)
	if err != nil {
		respondError(w, http.StatusNotFound, "Product not found")
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

func (h *PostgresHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	if err := h.Store.Delete(id); err != nil {
		respondError(w, http.StatusNotFound, "Product not found")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"result": "success"})
}
