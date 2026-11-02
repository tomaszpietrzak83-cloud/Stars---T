package db

import (
	"context"
	"database/sql"

	"github.com/sirgwain/craig-stars/db/generated"
)

// turnStatements belongs to one serial full-game save and its transaction.
// Reuse SQL compilation while preserving every write, argument and write order.
// Queries continue through the original transaction; only Exec is cached.
type turnStatements struct {
	generated.DBTX
	statements map[string]*sql.Stmt
}

func (s *turnStatements) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	stmt := s.statements[query]
	if stmt == nil {
		var err error
		stmt, err = s.DBTX.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		if s.statements == nil {
			s.statements = make(map[string]*sql.Stmt)
		}
		s.statements[query] = stmt
	}
	return stmt.ExecContext(ctx, args...)
}

func (s *turnStatements) close() {
	for _, stmt := range s.statements {
		stmt.Close()
	}
}
