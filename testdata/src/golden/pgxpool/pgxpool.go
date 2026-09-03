package pgxpool

import "context"

type Pool struct{}

func (p *Pool) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) any {
	return nil
}

func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}
