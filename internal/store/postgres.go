package store

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/mrckurz/CI-CD-MCM/internal/model"
)

// PostgresStore implements product storage backed by PostgreSQL.
type PostgresStore struct {
	DB *sql.DB
}

// NewPostgresStore creates a new PostgreSQL-backed store.
func NewPostgresStore(host, port, user, password, dbname string) (*PostgresStore, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &PostgresStore{DB: db}, nil
}

// EnsureTable creates the products table if it does not exist.
func (s *PostgresStore) EnsureTable() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS products (
			id    SERIAL PRIMARY KEY,
			name  TEXT NOT NULL,
			price NUMERIC(10,2) NOT NULL DEFAULT 0
		)
	`)
	return err
}

// GetAll returns all products.
func (s *PostgresStore) GetAll() ([]model.Product, error) {
	rows, err := s.DB.Query("SELECT id, name, price FROM products ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if products == nil {
		products = []model.Product{}
	}
	return products, nil
}

// GetByID returns a product by ID.
func (s *PostgresStore) GetByID(id int) (model.Product, error) {
	var p model.Product
	err := s.DB.QueryRow("SELECT id, name, price FROM products WHERE id = $1", id).
		Scan(&p.ID, &p.Name, &p.Price)
	if err == sql.ErrNoRows {
		return p, ErrNotFound
	}
	return p, err
}

// Create inserts a new product.
func (s *PostgresStore) Create(p model.Product) (model.Product, error) {
	err := s.DB.QueryRow(
		"INSERT INTO products (name, price) VALUES ($1, $2) RETURNING id",
		p.Name, p.Price,
	).Scan(&p.ID)
	return p, err
}

// Update modifies an existing product.
func (s *PostgresStore) Update(id int, p model.Product) (model.Product, error) {
	result, err := s.DB.Exec(
		"UPDATE products SET name = $1, price = $2 WHERE id = $3",
		p.Name, p.Price, id,
	)
	if err != nil {
		return p, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return p, ErrNotFound
	}
	p.ID = id
	return p, nil
}

// Delete removes a product by ID.
func (s *PostgresStore) Delete(id int) error {
	result, err := s.DB.Exec("DELETE FROM products WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
