package a25

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

var argon2 = struct {
	IDKey func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte
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

// 1. Safe computation outside transaction (Compliant)
func SafeOutsideTx(ctx context.Context, pool Pool, password string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	return pool.BeginFunc(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "INSERT INTO users (hash) VALUES ($1)", hashed)
		return err
	})
}

// 2. Safe light string/math transformations inside transaction (Compliant)
func SafeLightTx(ctx context.Context, pool Pool, name string) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		clean := strings.ToLower(strings.TrimSpace(name))
		_, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", clean)
		return err
	})
}

// 3. Unsafe bcrypt hashing inside BeginFunc (Violation)
func UnsafeBcryptInClosure(ctx context.Context, pool Pool, password string) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12) // want `\[ARGUS-A25\] cryptographic password hashing \(bcrypt\) inside active database transaction; risks connection pool starvation and lock convoy \(CWE-400, CWE-662\)`
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO users (hash) VALUES ($1)", hashed)
		return err
	})
}

// 4. Unsafe argon2 key derivation inside explicit transaction (Violation)
func UnsafeArgon2InExplicitTx(ctx context.Context, pool Pool, password, salt []byte) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	derived := argon2.IDKey(password, salt, 1, 64*1024, 4, 32) // want `\[ARGUS-A25\] cryptographic key derivation \(argon2\) inside active database transaction; risks connection pool starvation and lock convoy \(CWE-400, CWE-662\)`
	if _, err := tx.Exec(ctx, "INSERT INTO keys (k) VALUES ($1)", derived); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// 5. Unsafe RSA keygen inside transaction (Violation)
func UnsafeRSAKeygenInTx(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		key, err := rsa.GenerateKey(rand.Reader, 2048) // want `\[ARGUS-A25\] asymmetric keypair generation \(rsa\.GenerateKey\) inside active database transaction; risks connection pool starvation and lock convoy \(CWE-400, CWE-662\)`
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO keys (n) VALUES ($1)", key.N.String())
		return err
	})
}

// 6. Unsafe exec.Command inside transaction (Violation)
func UnsafeExecInTx(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		cmd := exec.Command("typst", "compile", "doc.typ") // want `\[ARGUS-A25\] external subprocess execution \(exec\.Command\) inside active database transaction; risks connection pool starvation and lock convoy \(CWE-400, CWE-662\)`
		_ = cmd.Run()
		_, err := tx.Exec(ctx, "UPDATE docs SET status = 'DONE'")
		return err
	})
}

// 7. Ignored via directive
func IgnoredBcrypt(ctx context.Context, pool Pool, password string) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		// argus:ignore ARGUS-A25 mock test harness isolated execution
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), 4)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO users (hash) VALUES ($1)", hashed)
		return err
	})
}

// 8. Ignored via shortcode
func IgnoredShortcode(ctx context.Context, pool Pool, password string) error {
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
