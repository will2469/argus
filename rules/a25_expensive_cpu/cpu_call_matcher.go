// Package a25_expensive_cpu identifies CPU-expensive computations (bcrypt, argon2, RSA keygen, exec).
package a25_expensive_cpu

import (
	"go/ast"
)

// MatchExpensiveCPUCall inspects a CallExpr to determine if it is a known CPU-expensive operation.
func MatchExpensiveCPUCall(call *ast.CallExpr) (isExpensive bool, reason string) {
	if call == nil {
		return false, ""
	}

	pkgName, funcName := extractPkgAndFunc(call.Fun)
	if pkgName == "" && funcName == "" {
		return false, ""
	}

	// 1. Password Hashing & Key Derivation
	switch pkgName {
	case "bcrypt":
		if funcName == "GenerateFromPassword" || funcName == "CompareHashAndPassword" {
			return true, "cryptographic password hashing (bcrypt) inside active database transaction; risks connection pool starvation and lock convoy (CWE-400, CWE-662)"
		}
	case "argon2":
		if funcName == "IDKey" || funcName == "Key" {
			return true, "cryptographic key derivation (argon2) inside active database transaction; risks connection pool starvation and lock convoy (CWE-400, CWE-662)"
		}
	case "scrypt":
		if funcName == "Key" {
			return true, "cryptographic key derivation (scrypt) inside active database transaction; risks connection pool starvation and lock convoy (CWE-400, CWE-662)"
		}
	case "rsa":
		if funcName == "GenerateKey" || funcName == "GenerateMultiPrimeKey" {
			return true, "asymmetric keypair generation (rsa.GenerateKey) inside active database transaction; risks connection pool starvation and lock convoy (CWE-400, CWE-662)"
		}
	case "ecdsa":
		if funcName == "GenerateKey" {
			return true, "asymmetric keypair generation (ecdsa.GenerateKey) inside active database transaction; risks connection pool starvation and lock convoy (CWE-400, CWE-662)"
		}
	case "ed25519":
		if funcName == "GenerateKey" {
			return true, "asymmetric keypair generation (ed25519.GenerateKey) inside active database transaction; risks connection pool starvation and lock convoy (CWE-400, CWE-662)"
		}
	case "exec":
		if funcName == "Command" || funcName == "CommandContext" {
			return true, "external subprocess execution (exec.Command) inside active database transaction; risks connection pool starvation and lock convoy (CWE-400, CWE-662)"
		}
	}

	return false, ""
}

func extractPkgAndFunc(expr ast.Expr) (pkg, fn string) {
	switch x := expr.(type) {
	case *ast.SelectorExpr:
		fn = x.Sel.Name
		if id, ok := x.X.(*ast.Ident); ok {
			pkg = id.Name
		}
	case *ast.Ident:
		fn = x.Name
	}
	return pkg, fn
}
