package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type statementTestValue string

func (v statementTestValue) Value() (driver.Value, error) {
	if v == "fail" {
		return nil, errors.New("encoding failed")
	}
	return string(v), nil
}

func TestTurnStatementsBindingErrorsAndRollback(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer database.Close()
	database.SetMaxOpenConns(1)
	_, err = database.Exec("CREATE TABLE saved (id INTEGER PRIMARY KEY, payload TEXT)")
	require.NoError(t, err)

	tx, err := database.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	s := &turnStatements{DBTX: tx}
	defer s.close()
	const insert = "INSERT INTO saved (id, payload) VALUES (?, ?)"
	_, err = s.ExecContext(ctx, insert, 1, statementTestValue("first"))
	require.NoError(t, err)
	_, err = s.ExecContext(ctx, insert, 2, nil)
	require.NoError(t, err)
	_, err = s.ExecContext(ctx, insert, 3, statementTestValue("fail"))
	require.ErrorContains(t, err, "encoding failed")
	_, err = s.ExecContext(ctx, insert, 1, "duplicate")
	require.Error(t, err)
	_, err = s.ExecContext(ctx, insert, 3, "third")
	require.NoError(t, err)
	require.Len(t, s.statements, 1)
	var payload sql.NullString
	require.NoError(t, s.QueryRowContext(ctx, "SELECT payload FROM saved WHERE id = 2").Scan(&payload))
	require.False(t, payload.Valid, "nil must not reuse an earlier binding")
	s.close()
	require.NoError(t, tx.Rollback())
	var count int
	require.NoError(t, database.QueryRow("SELECT count(*) FROM saved").Scan(&count))
	require.Zero(t, count)

	// Statements may never escape into a later transaction.
	tx2, err := database.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx2.Rollback()
	s2 := &turnStatements{DBTX: tx2}
	_, err = s2.ExecContext(ctx, insert, 1, "new transaction")
	require.NoError(t, err)
	s2.close()
	require.NoError(t, tx2.Commit())
	require.NoError(t, database.QueryRow("SELECT payload FROM saved WHERE id = 1").Scan(&payload))
	require.Equal(t, "new transaction", payload.String)
}
