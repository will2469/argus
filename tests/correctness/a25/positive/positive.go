package positive

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"os/exec"
)

var bcrypt = struct {
	GenerateFromPassword func(password []byte, cost int) ([]byte, error)
}{}

var argon2 = struct {
	IDKey func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte
}{}

var scrypt = struct {
	Key func(password, salt []byte, N, r, p, keyLen int) ([]byte, error)
}{}

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Pool interface {
	Begin(ctx context.Context) (DB, error)
	BeginFunc(ctx context.Context, fn func(tx DB) error) error
}

// P1: Obvious Violation — bcrypt password hashing inside BeginFunc.
func P1_Obvious(ctx context.Context, pool Pool, password string) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12) // want `\[ARGUS-A25\] cryptographic password hashing \(bcrypt\) inside active database transaction; risks connection pool starvation and lock convoy \(CWE-400, CWE-662\)`
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO users (hash) VALUES ($1)", hashed)
		return err
	})
}

// P2: Indirect Violation — argon2 key derivation inside explicit transaction.
func P2_Indirect(ctx context.Context, pool Pool, password, salt []byte) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	derived := argon2.IDKey(password, salt, 1, 64*1024, 4, 32) // want `\[ARGUS-A25\] cryptographic key derivation \(argon2\) inside active database transaction; risks connection pool starvation and lock convoy \(CWE-400, CWE-662\)`
	if _, err := tx.Exec(ctx, "INSERT INTO keys (k) VALUES ($1)", derived); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// P3: Helper Violation — RSA keygen inside BeginFunc.
func P3_Helper(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		key, err := rsa.GenerateKey(rand.Reader, 2048) // want `\[ARGUS-A25\] asymmetric keypair generation \(rsa\.GenerateKey\) inside active database transaction; risks connection pool starvation and lock convoy \(CWE-400, CWE-662\)`
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO keys (n) VALUES ($1)", key.N.String())
		return err
	})
}

// P4: Nested Violation — Subprocess execution inside BeginFunc.
func P4_Nested(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		cmd := exec.Command("typst", "compile", "doc.typ") // want `\[ARGUS-A25\] external subprocess execution \(exec\.Command\) inside active database transaction; risks connection pool starvation and lock convoy \(CWE-400, CWE-662\)`
		_ = cmd.Run()
		_, err := tx.Exec(ctx, "UPDATE docs SET status = 'DONE'")
		return err
	})
}

// P5: Alias Violation — scrypt key derivation inside explicit transaction.
func P5_Alias(ctx context.Context, pool Pool, password string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	k, _ := scrypt.Key([]byte(password), []byte("salt"), 16384, 8, 1, 32) // want `\[ARGUS-A25\] cryptographic key derivation \(scrypt\) inside active database transaction; risks connection pool starvation and lock convoy \(CWE-400, CWE-662\)`
	_, err = tx.Exec(ctx, "INSERT INTO secrets (s) VALUES ($1)", k)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context, pool Pool, password string) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		// argus:ignore-a25 legacy migration pipeline
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), 4)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO users (hash) VALUES ($1)", hashed)
		return err
	})
}
