package store

import (
	"testing"

	"github.com/mrckurz/CI-CD-MCM/internal/model"
)

func TestCreateAndGet(t *testing.T) {
	s := NewMemoryStore()

	// Create a product
	product := model.Product{
		Name:  "Test Product",
		Price: 29.99,
	}

	created := s.Create(product)

	// Verify it was assigned an ID
	if created.ID != 1 {
		t.Errorf("expected ID 1, got %d", created.ID)
	}

	// Retrieve it and verify
	retrieved, err := s.GetByID(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.Name != created.Name {
		t.Errorf("expected name %q, got %q", created.Name, retrieved.Name)
	}
	if retrieved.Price != created.Price {
		t.Errorf("expected price %f, got %f", created.Price, retrieved.Price)
	}
}

func TestUpdateProduct(t *testing.T) {
	s := NewMemoryStore()

	// Create initial product
	original := model.Product{
		Name:  "Original Name",
		Price: 10.00,
	}
	created := s.Create(original)

	// Table-driven tests for various update scenarios
	tests := []struct {
		name        string
		id          int
		newProduct  model.Product
		expectError bool
		expectName  string
		expectPrice float64
	}{
		{
			name: "update existing product name and price",
			id:   created.ID,
			newProduct: model.Product{
				Name:  "Updated Name",
				Price: 20.00,
			},
			expectError: false,
			expectName:  "Updated Name",
			expectPrice: 20.00,
		},
		{
			name: "update existing product name only",
			id:   created.ID,
			newProduct: model.Product{
				Name:  "Another Name",
				Price: 20.00,
			},
			expectError: false,
			expectName:  "Another Name",
			expectPrice: 20.00,
		},
		{
			name: "update non-existent product returns error",
			id:   999,
			newProduct: model.Product{
				Name:  "Some Product",
				Price: 15.00,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, err := s.Update(tt.id, tt.newProduct)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !tt.expectError {
				if updated.Name != tt.expectName {
					t.Errorf("expected name %q, got %q", tt.expectName, updated.Name)
				}
				if updated.Price != tt.expectPrice {
					t.Errorf("expected price %f, got %f", tt.expectPrice, updated.Price)
				}

				// Verify update persisted in store
				retrieved, _ := s.GetByID(tt.id)
				if retrieved.Name != tt.expectName {
					t.Errorf("expected stored name %q, got %q", tt.expectName, retrieved.Name)
				}
				if retrieved.Price != tt.expectPrice {
					t.Errorf("expected stored price %f, got %f", tt.expectPrice, retrieved.Price)
				}
			}
		})
	}
}

func TestDeleteProduct(t *testing.T) {
	s := NewMemoryStore()

	// Create a product
	product := model.Product{
		Name:  "To Delete",
		Price: 5.00,
	}
	created := s.Create(product)

	// Verify it exists before deletion
	_, err := s.GetByID(created.ID)
	if err != nil {
		t.Fatalf("product should exist before deletion")
	}

	// Delete it
	err = s.Delete(created.ID)
	if err != nil {
		t.Fatalf("unexpected error deleting product: %v", err)
	}

	// Verify it's gone
	_, err = s.GetByID(created.ID)
	if err != ErrNotFound {
		t.Error("expected ErrNotFound after deletion")
	}
}

func TestGetByIDNotFound(t *testing.T) {
	s := NewMemoryStore()

	// Try to get a non-existent product
	_, err := s.GetByID(999)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetAllEmpty(t *testing.T) {
	s := NewMemoryStore()
	products := s.GetAll()
	if len(products) != 0 {
		t.Errorf("expected 0 products, got %d", len(products))
	}
}

func TestDeleteNonExistent(t *testing.T) {
	s := NewMemoryStore()
	err := s.Delete(999)
	if err != ErrNotFound {
		t.Error("expected ErrNotFound when deleting non-existent product")
	}
}
