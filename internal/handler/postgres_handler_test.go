package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/mux"
	"github.com/mrckurz/CI-CD-MCM/internal/store"
)

const fakeHandlerPostgresDriverName = "fake-handler-postgres"

var (
	registerFakeHandlerPostgres sync.Once
	activeHandlerScript         *fakeHandlerPostgresScript
)

type fakeHandlerPostgresScript struct {
	mu      sync.Mutex
	pingErr error
	execs   []fakeHandlerExec
	queries []fakeHandlerQuery
}

type fakeHandlerExec struct {
	rowsAffected int64
	err          error
}

type fakeHandlerQuery struct {
	columns []string
	rows    [][]driver.Value
	err     error
}

type fakeHandlerPostgresDriver struct{}

func (d fakeHandlerPostgresDriver) Open(_ string) (driver.Conn, error) {
	return fakeHandlerPostgresConn{}, nil
}

type fakeHandlerPostgresConn struct{}

func (c fakeHandlerPostgresConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not supported by fake driver")
}

func (c fakeHandlerPostgresConn) Close() error {
	return nil
}

func (c fakeHandlerPostgresConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported by fake driver")
}

func (c fakeHandlerPostgresConn) Ping(_ context.Context) error {
	activeHandlerScript.mu.Lock()
	defer activeHandlerScript.mu.Unlock()
	return activeHandlerScript.pingErr
}

func (c fakeHandlerPostgresConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	activeHandlerScript.mu.Lock()
	defer activeHandlerScript.mu.Unlock()

	if len(activeHandlerScript.execs) == 0 {
		return nil, errors.New("unexpected exec")
	}
	next := activeHandlerScript.execs[0]
	activeHandlerScript.execs = activeHandlerScript.execs[1:]
	if next.err != nil {
		return nil, next.err
	}
	return fakeHandlerResult{rowsAffected: next.rowsAffected}, nil
}

func (c fakeHandlerPostgresConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	activeHandlerScript.mu.Lock()
	defer activeHandlerScript.mu.Unlock()

	if len(activeHandlerScript.queries) == 0 {
		return nil, errors.New("unexpected query")
	}
	next := activeHandlerScript.queries[0]
	activeHandlerScript.queries = activeHandlerScript.queries[1:]
	if next.err != nil {
		return nil, next.err
	}
	return &fakeHandlerRows{columns: next.columns, rows: next.rows}, nil
}

type fakeHandlerResult struct {
	rowsAffected int64
}

func (r fakeHandlerResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r fakeHandlerResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

type fakeHandlerRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *fakeHandlerRows) Columns() []string {
	return r.columns
}

func (r *fakeHandlerRows) Close() error {
	return nil
}

func (r *fakeHandlerRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func setupPostgresRouter(t *testing.T, script *fakeHandlerPostgresScript) *mux.Router {
	t.Helper()

	registerFakeHandlerPostgres.Do(func() {
		sql.Register(fakeHandlerPostgresDriverName, fakeHandlerPostgresDriver{})
	})
	activeHandlerScript = script

	db, err := sql.Open(fakeHandlerPostgresDriverName, "")
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})

	h := NewPostgresHandler(&store.PostgresStore{DB: db})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func handlerProductColumns() []string {
	return []string{"id", "name", "price"}
}

func assertPostgresStatus(t *testing.T, r *mux.Router, method, path, body string, want int) {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != want {
		t.Fatalf("expected %d, got %d: %s", want, rr.Code, rr.Body.String())
	}
}

func TestPostgresHandlerHealth(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{})

	assertPostgresStatus(t, r, "GET", "/health", "", http.StatusOK)
}

func TestPostgresHandlerHealthUnavailable(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{pingErr: errors.New("ping failed")})

	assertPostgresStatus(t, r, "GET", "/health", "", http.StatusServiceUnavailable)
}

func TestPostgresHandlerGetProducts(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		queries: []fakeHandlerQuery{{
			columns: handlerProductColumns(),
			rows:    [][]driver.Value{{int64(1), "Widget", float64(9.99)}},
		}},
	})

	assertPostgresStatus(t, r, "GET", "/products", "", http.StatusOK)
}

func TestPostgresHandlerGetProductsError(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		queries: []fakeHandlerQuery{{err: errors.New("query failed")}},
	})

	assertPostgresStatus(t, r, "GET", "/products", "", http.StatusInternalServerError)
}

func TestPostgresHandlerGetProduct(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		queries: []fakeHandlerQuery{{
			columns: handlerProductColumns(),
			rows:    [][]driver.Value{{int64(1), "Widget", float64(9.99)}},
		}},
	})

	assertPostgresStatus(t, r, "GET", "/products/1", "", http.StatusOK)
}

func TestPostgresHandlerGetProductNotFound(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		queries: []fakeHandlerQuery{{columns: handlerProductColumns()}},
	})

	assertPostgresStatus(t, r, "GET", "/products/999", "", http.StatusNotFound)
}

func TestPostgresHandlerCreateProduct(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		queries: []fakeHandlerQuery{{
			columns: []string{"id"},
			rows:    [][]driver.Value{{int64(1)}},
		}},
	})

	assertPostgresStatus(t, r, "POST", "/products", `{"name":"Widget","price":9.99}`, http.StatusCreated)
}

func TestPostgresHandlerCreateProductRejectsMalformedJSON(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{})

	assertPostgresStatus(t, r, "POST", "/products", `{"name":`, http.StatusBadRequest)
}

func TestPostgresHandlerCreateProductRejectsInvalidProduct(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{})

	assertPostgresStatus(t, r, "POST", "/products", `{"name":"","price":1}`, http.StatusBadRequest)
}

func TestPostgresHandlerCreateProductStoreError(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		queries: []fakeHandlerQuery{{err: errors.New("insert failed")}},
	})

	assertPostgresStatus(t, r, "POST", "/products", `{"name":"Widget","price":9.99}`, http.StatusInternalServerError)
}

func TestPostgresHandlerUpdateProduct(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		execs: []fakeHandlerExec{{rowsAffected: 1}},
	})

	assertPostgresStatus(t, r, "PUT", "/products/1", `{"name":"Updated","price":12.5}`, http.StatusOK)
}

func TestPostgresHandlerUpdateProductRejectsMalformedJSON(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{})

	assertPostgresStatus(t, r, "PUT", "/products/1", `{"name":`, http.StatusBadRequest)
}

func TestPostgresHandlerUpdateProductNotFound(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		execs: []fakeHandlerExec{{rowsAffected: 0}},
	})

	assertPostgresStatus(t, r, "PUT", "/products/999", `{"name":"Missing","price":1}`, http.StatusNotFound)
}

func TestPostgresHandlerDeleteProduct(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		execs: []fakeHandlerExec{{rowsAffected: 1}},
	})

	assertPostgresStatus(t, r, "DELETE", "/products/1", "", http.StatusOK)
}

func TestPostgresHandlerDeleteProductNotFound(t *testing.T) {
	r := setupPostgresRouter(t, &fakeHandlerPostgresScript{
		execs: []fakeHandlerExec{{rowsAffected: 0}},
	})

	assertPostgresStatus(t, r, "DELETE", "/products/999", "", http.StatusNotFound)
}
