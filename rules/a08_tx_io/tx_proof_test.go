package a08_tx_io

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestCustomInterfaceWithBegin_MustNotBeDBPool(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"context"
	"net/http"
	"time"
)

type WorkflowRunner interface {
	Begin(ctx context.Context) (WorkflowRun, error)
}

type WorkflowRun interface {
	Status() string
}

func RunWorkflow(ctx context.Context, runner WorkflowRunner) {
	run, _ := runner.Begin(ctx)
	time.Sleep(10 * time.Millisecond)
	_, _ = http.Get("https://api.example.com/status")
	_ = run
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for custom WorkflowRunner.Begin interface, got %d: %v", len(issues), issues)
	}
}

func TestCustomVideoPlayerInterface_MustNotBeDBPool(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"net/http"
)

type VideoPlayer interface {
	Begin()
}

func PlayVideo(player VideoPlayer) {
	player.Begin()
	_, _ = http.Get("https://example.com/video")
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for custom VideoPlayer.Begin interface, got %d: %v", len(issues), issues)
	}
}

func TestUnresolvedIdentifier_FailsClosedAsNotTx(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"time"
)

func Process(unknownObject any) {
	// Unresolved identifier or unknown type must NOT be treated as a database transaction
	time.Sleep(10 * time.Millisecond)
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for unresolved identifier, got %d: %v", len(issues), issues)
	}
}
