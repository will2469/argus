package adversarial

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
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

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

type Pool interface {
	BeginFunc(ctx context.Context, fn func(tx DB) error) error
}

// A1: Branch — conditional expensive operation within transaction.
func A1_Branch(ctx context.Context, pool Pool, password string, mustHash bool) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		if mustHash {
			_, _ = bcrypt.GenerateFromPassword([]byte(password), 10)
		}
		return nil
	})
}

// Helper called by A2
func hashPasswordHelper(pw string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(pw), 12)
}

// A2: Reassignment/Helper — helper function called inside transaction.
func A2_Reassignment(ctx context.Context, pool Pool, pw string) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		_, _ = hashPasswordHelper(pw)
		return nil
	})
}

// A3: Alias — closure assigned to variable before being passed to BeginFunc.
func A3_Alias(ctx context.Context, pool Pool) error {
	fn := func(tx DB) error {
		_ = exec.Command("ls", "-la")
		return nil
	}
	return pool.BeginFunc(ctx, fn)
}

// A4: Wrapper — struct method performing transaction with RSA keygen.
type KeyManager struct {
	pool Pool
}

func (m KeyManager) GenerateInTx(ctx context.Context) error {
	return m.pool.BeginFunc(ctx, func(tx DB) error {
		_, _ = rsa.GenerateKey(rand.Reader, 2048)
		return nil
	})
}

// A5: Nested Function — nested closure executing argon2 inside transaction.
func A5_NestedFunction(ctx context.Context, pool Pool, pw, salt []byte) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		inner := func() {
			_ = argon2.IDKey(pw, salt, 1, 64*1024, 4, 32)
		}
		inner()
		return nil
	})
}

// A6: Generic — generic transaction runner with exec.Command.
type TxRunner[T any] struct {
	pool Pool
}

func (r TxRunner[T]) Run(ctx context.Context) error {
	return r.pool.BeginFunc(ctx, func(tx DB) error {
		_ = exec.Command("echo", "test")
		return nil
	})
}

// A7: ECDSA Keygen — asymmetric keypair generation inside transaction.
func A7_ECDSAKeygen(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx DB) error {
		_, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		return nil
	})
}
