package negative

import (
	"context"
)

type DB struct{}

func (DB) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func (DB) QueryRow(ctx context.Context, sql string, args ...any) any {
	return nil
}

type NonDBWorker struct{}

func (NonDBWorker) Exec(cmd string) error {
	return nil
}

type SearchEngine struct{}

func (SearchEngine) Query(ctx context.Context, q string) string {
	return q
}

// N1: Obvious Safe — set-based query with ANY($1) outside loop.
func N1_ObviousSafe(ctx context.Context, db DB, ids []int) {
	_, _ = db.Query(ctx, "SELECT email FROM users WHERE id = ANY($1)", ids)
}

// N2: Legitimate Idiom — constant retry loop (bounded integer range).
func N2_LegitimateIdiom(ctx context.Context, db DB) {
	for range 3 {
		_ = db.QueryRow(ctx, "SELECT 1")
	}
}

// N3: Unrelated API — SearchEngine.Query called inside loop.
func N3_UnrelatedAPI(ctx context.Context, search SearchEngine, items []string) {
	for _, item := range items {
		_ = search.Query(ctx, item)
	}
}

// N4: Non-DB Worker — worker.Exec called inside loop.
func N4_NonDBWorker(worker NonDBWorker, ids []int) {
	for range ids {
		_ = worker.Exec("echo hello")
	}
}

// N5: Static Computation — in-memory processing loop without DB calls.
func N5_StaticComputation(ids []int) []int {
	results := make([]int, len(ids))
	for i, id := range ids {
		results[i] = id * 2
	}
	return results
}

type MemoryCache struct{}

func (MemoryCache) Get(id int) string {
	return "cached"
}

type DBRepo struct{}

func (DBRepo) Get(ctx context.Context, db DB, id int) {
	_ = db.QueryRow(ctx, "SELECT 1 WHERE id = $1", id)
}

// N6: Receiver Collision Safety — MemoryCache.Get called inside loop must NOT be flagged as N+1 even though DBRepo also has a Get method.
func N6_ReceiverCollisionSafety(cache MemoryCache, ids []int) {
	for _, id := range ids {
		_ = cache.Get(id)
	}
}
