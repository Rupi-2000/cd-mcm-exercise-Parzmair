package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/mrckurz/CI-CD-MCM/internal/model"
)

const fakePostgresDriverName = "fake-postgres"

var (
	registerFakePostgres sync.Once
	activeFakeScript     *fakePostgresScript
)

type fakePostgresScript struct {
	mu      sync.Mutex
	pingErr error
	execs   []fakeExec
	queries []fakeQuery
}

type fakeExec struct {
	rowsAffected    int64
	err             error
	rowsAffectedErr error
}

type fakeQuery struct {
	columns []string
	rows    [][]driver.Value
	err     error
	rowsErr error
}

type fakePostgresDriver struct{}

func (d fakePostgresDriver) Open(_ string) (driver.Conn, error) {
	return fakePostgresConn{}, nil
}

type fakePostgresConn struct{}

func (c fakePostgresConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not supported by fake driver")
}

func (c fakePostgresConn) Close() error {
	return nil
}

func (c fakePostgresConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported by fake driver")
}

func (c fakePostgresConn) Ping(_ context.Context) error {
	activeFakeScript.mu.Lock()
	defer activeFakeScript.mu.Unlock()
	return activeFakeScript.pingErr
}

func (c fakePostgresConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	activeFakeScript.mu.Lock()
	defer activeFakeScript.mu.Unlock()

	if len(activeFakeScript.execs) == 0 {
		return nil, errors.New("unexpected exec")
	}
	next := activeFakeScript.execs[0]
	activeFakeScript.execs = activeFakeScript.execs[1:]
	if next.err != nil {
		return nil, next.err
	}
	return fakeResult{rowsAffected: next.rowsAffected, rowsAffectedErr: next.rowsAffectedErr}, nil
}

func (c fakePostgresConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	activeFakeScript.mu.Lock()
	defer activeFakeScript.mu.Unlock()

	if len(activeFakeScript.queries) == 0 {
		return nil, errors.New("unexpected query")
	}
	next := activeFakeScript.queries[0]
	activeFakeScript.queries = activeFakeScript.queries[1:]
	if next.err != nil {
		return nil, next.err
	}
	return &fakeRows{columns: next.columns, rows: next.rows, rowsErr: next.rowsErr}, nil
}

type fakeResult struct {
	rowsAffected    int64
	rowsAffectedErr error
}

func (r fakeResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r fakeResult) RowsAffected() (int64, error) {
	if r.rowsAffectedErr != nil {
		return 0, r.rowsAffectedErr
	}
	return r.rowsAffected, nil
}

type fakeRows struct {
	columns []string
	rows    [][]driver.Value
	rowsErr error
	index   int
}

func (r *fakeRows) Columns() []string {
	return r.columns
}

func (r *fakeRows) Close() error {
	return nil
}

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		if r.rowsErr != nil {
			err := r.rowsErr
			r.rowsErr = nil
			return err
		}
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func newFakePostgresStore(t *testing.T, script *fakePostgresScript) *PostgresStore {
	t.Helper()

	registerFakePostgres.Do(func() {
		sql.Register(fakePostgresDriverName, fakePostgresDriver{})
	})
	activeFakeScript = script

	db, err := sql.Open(fakePostgresDriverName, "")
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})

	return &PostgresStore{DB: db}
}

func productColumns() []string {
	return []string{"id", "name", "price"}
}

func TestPostgresEnsureTable(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		execs: []fakeExec{{rowsAffected: 0}},
	})

	if err := s.EnsureTable(); err != nil {
		t.Fatalf("EnsureTable returned error: %v", err)
	}
}

func TestPostgresGetAllReturnsProducts(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		queries: []fakeQuery{{
			columns: productColumns(),
			rows: [][]driver.Value{
				{int64(1), "Widget", float64(9.99)},
				{int64(2), "Gadget", float64(14.99)},
			},
		}},
	})

	products, err := s.GetAll()
	if err != nil {
		t.Fatalf("GetAll returned error: %v", err)
	}
	if len(products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(products))
	}
}

func TestPostgresGetAllReturnsEmptySlice(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		queries: []fakeQuery{{columns: productColumns()}},
	})

	products, err := s.GetAll()
	if err != nil {
		t.Fatalf("GetAll returned error: %v", err)
	}
	if products == nil || len(products) != 0 {
		t.Fatalf("expected empty non-nil slice, got %+v", products)
	}
}

func TestPostgresGetAllReturnsRowsError(t *testing.T) {
	rowsErr := errors.New("rows failed")
	s := newFakePostgresStore(t, &fakePostgresScript{
		queries: []fakeQuery{{columns: productColumns(), rowsErr: rowsErr}},
	})

	_, err := s.GetAll()
	if !errors.Is(err, rowsErr) {
		t.Fatalf("expected rows error, got %v", err)
	}
}

func TestPostgresGetByID(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		queries: []fakeQuery{{
			columns: productColumns(),
			rows:    [][]driver.Value{{int64(1), "Widget", float64(9.99)}},
		}},
	})

	p, err := s.GetByID(1)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if p.ID != 1 || p.Name != "Widget" || p.Price != 9.99 {
		t.Fatalf("unexpected product: %+v", p)
	}
}

func TestPostgresGetByIDNotFound(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		queries: []fakeQuery{{columns: productColumns()}},
	})

	_, err := s.GetByID(999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresCreate(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		queries: []fakeQuery{{
			columns: []string{"id"},
			rows:    [][]driver.Value{{int64(7)}},
		}},
	})

	created, err := s.Create(model.Product{Name: "Widget", Price: 9.99})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID != 7 {
		t.Fatalf("expected ID 7, got %d", created.ID)
	}
}

func TestPostgresCreateReturnsError(t *testing.T) {
	queryErr := errors.New("insert failed")
	s := newFakePostgresStore(t, &fakePostgresScript{
		queries: []fakeQuery{{err: queryErr}},
	})

	_, err := s.Create(model.Product{Name: "Widget", Price: 9.99})
	if !errors.Is(err, queryErr) {
		t.Fatalf("expected insert error, got %v", err)
	}
}

func TestPostgresUpdate(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		execs: []fakeExec{{rowsAffected: 1}},
	})

	updated, err := s.Update(3, model.Product{Name: "Updated", Price: 12.5})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.ID != 3 || updated.Name != "Updated" || updated.Price != 12.5 {
		t.Fatalf("unexpected product: %+v", updated)
	}
}

func TestPostgresUpdateNotFound(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		execs: []fakeExec{{rowsAffected: 0}},
	})

	_, err := s.Update(999, model.Product{Name: "Missing", Price: 1})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresUpdateRowsAffectedError(t *testing.T) {
	rowsErr := errors.New("rows affected failed")
	s := newFakePostgresStore(t, &fakePostgresScript{
		execs: []fakeExec{{rowsAffectedErr: rowsErr}},
	})

	_, err := s.Update(1, model.Product{Name: "Widget", Price: 1})
	if !errors.Is(err, rowsErr) {
		t.Fatalf("expected rows affected error, got %v", err)
	}
}

func TestPostgresDelete(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		execs: []fakeExec{{rowsAffected: 1}},
	})

	if err := s.Delete(1); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestPostgresDeleteNotFound(t *testing.T) {
	s := newFakePostgresStore(t, &fakePostgresScript{
		execs: []fakeExec{{rowsAffected: 0}},
	})

	err := s.Delete(999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresDeleteReturnsExecError(t *testing.T) {
	execErr := errors.New("delete failed")
	s := newFakePostgresStore(t, &fakePostgresScript{
		execs: []fakeExec{{err: execErr}},
	})

	err := s.Delete(1)
	if !errors.Is(err, execErr) {
		t.Fatalf("expected exec error, got %v", err)
	}
}

func TestPostgresDeleteRowsAffectedError(t *testing.T) {
	rowsErr := errors.New("rows affected failed")
	s := newFakePostgresStore(t, &fakePostgresScript{
		execs: []fakeExec{{rowsAffectedErr: rowsErr}},
	})

	err := s.Delete(1)
	if !errors.Is(err, rowsErr) {
		t.Fatalf("expected rows affected error, got %v", err)
	}
}
