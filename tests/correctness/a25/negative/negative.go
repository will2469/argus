package negative

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"os/exec"
	"strings"
)

var bcrypt = struct {
	GenerateFromPassword func(password []byte, cost int) ([]byte, error)
}{}

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

type Pool interface {
	BeginFunc(ctx context.Context, fn func(tx DB) error) error
}

// N1: Obvious Safe — bcrypt password hashing outside transaction before BeginFunc.
func N1_ObviousSafe(ctx context.Context, pool Pool, password string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	return pool.BeginFunc(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "INSERT INTO users (hash) VALUES ($1)", hashed)
		return err
	})
}

// N2: Legitimate Idiom — Light string transformation inside transaction.
func N2_LegitimateIdiom(ctx context.Context, pool Pool, name string) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		clean := strings.ToLower(strings.TrimSpace(name))
		_, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", clean)
		return err
	})
}

// N3: Unrelated API — Normal database transaction without CPU calls.
func N3_UnrelatedAPI(ctx context.Context, pool Pool, id string) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "UPDATE users SET status = 'ACTIVE' WHERE id = $1", id)
		return err
	})
}

// N4: Keygen Outside Tx — Asymmetric keypair generated prior to transaction.
func N4_KeygenOutsideTx(ctx context.Context, pool Pool) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	return pool.BeginFunc(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "INSERT INTO keys (n) VALUES ($1)", key.N.String())
		return err
	})
}

// N5: Subprocess Outside Tx — Subprocess executed before transaction opens.
func N5_SubprocessOutsideTx(ctx context.Context, pool Pool) error {
	cmd := exec.Command("typst", "compile", "doc.typ")
	_ = cmd.Run()
	return pool.BeginFunc(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "UPDATE docs SET status = 'DONE'")
		return err
	})
}
