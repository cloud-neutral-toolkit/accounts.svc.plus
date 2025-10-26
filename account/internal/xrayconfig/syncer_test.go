package xrayconfig

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type staticSource struct {
	clients []Client
	err     error
}

func (s staticSource) ListClients(context.Context) ([]Client, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]Client(nil), s.clients...), nil
}

func writeTemplate(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.json")
	if err := os.WriteFile(templatePath, []byte(`{"inbounds":[{"settings":{"clients":[]}}]}`), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	outputPath := filepath.Join(dir, "config.json")
	return templatePath, outputPath
}

func TestNewPeriodicSyncerValidation(t *testing.T) {
	template, output := writeTemplate(t)
	tests := []struct {
		name string
		opts PeriodicOptions
	}{
		{
			name: "missing source",
			opts: PeriodicOptions{
				Interval:  time.Minute,
				Generator: Generator{TemplatePath: template, OutputPath: output},
			},
		},
		{
			name: "missing template",
			opts: PeriodicOptions{
				Interval:  time.Minute,
				Source:    staticSource{},
				Generator: Generator{OutputPath: output},
			},
		},
		{
			name: "missing output",
			opts: PeriodicOptions{
				Interval:  time.Minute,
				Source:    staticSource{},
				Generator: Generator{TemplatePath: template},
			},
		},
		{
			name: "non-positive interval",
			opts: PeriodicOptions{
				Interval:  0,
				Source:    staticSource{},
				Generator: Generator{TemplatePath: template, OutputPath: output},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewPeriodicSyncer(tt.opts); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestPeriodicSyncerSyncSuccess(t *testing.T) {
	template, output := writeTemplate(t)
	opts := PeriodicOptions{
		Interval:        time.Minute,
		Source:          staticSource{clients: []Client{{ID: "uuid-a", Email: "a@example"}, {ID: "uuid-b"}}},
		Generator:       Generator{TemplatePath: template, OutputPath: output},
		ValidateCommand: []string{"echo", "validate"},
		RestartCommand:  []string{"echo", "restart"},
	}

	var commands [][]string
	opts.Runner = func(_ context.Context, cmd []string) ([]byte, error) {
		commands = append(commands, append([]string(nil), cmd...))
		return []byte(strings.Join(cmd, " ")), nil
	}

	syncer, err := NewPeriodicSyncer(opts)
	if err != nil {
		t.Fatalf("new syncer: %v", err)
	}

	n, err := syncer.sync(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 clients, got %d", n)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "uuid-a") || !strings.Contains(string(data), "uuid-b") {
		t.Fatalf("output missing client ids: %s", string(data))
	}
	if got, want := len(commands), 2; got != want {
		t.Fatalf("expected %d commands, got %d", want, got)
	}
	if commands[0][0] != "echo" || commands[1][0] != "echo" {
		t.Fatalf("unexpected commands: %+v", commands)
	}
}

func TestPeriodicSyncerSyncError(t *testing.T) {
	template, output := writeTemplate(t)
	opts := PeriodicOptions{
		Interval:  time.Minute,
		Source:    staticSource{err: errors.New("boom")},
		Generator: Generator{TemplatePath: template, OutputPath: output},
	}
	syncer, err := NewPeriodicSyncer(opts)
	if err != nil {
		t.Fatalf("new syncer: %v", err)
	}
	if _, err := syncer.sync(context.Background()); err == nil {
		t.Fatalf("expected sync error")
	}
}

func TestPeriodicSyncerStartStop(t *testing.T) {
	template, output := writeTemplate(t)
	var calls atomic.Int32
	src := staticSource{clients: []Client{{ID: "uuid-a"}}}
	opts := PeriodicOptions{
		Interval: 10 * time.Millisecond,
		Source: clientSourceFunc(func(ctx context.Context) ([]Client, error) {
			calls.Add(1)
			return src.ListClients(ctx)
		}),
		Generator: Generator{TemplatePath: template, OutputPath: output},
	}
	syncer, err := NewPeriodicSyncer(opts)
	if err != nil {
		t.Fatalf("new syncer: %v", err)
	}
	stop, err := syncer.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := time.Now().Add(200 * time.Millisecond)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if calls.Load() == 0 {
		t.Fatalf("sync never executed")
	}
	if err := stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

type clientSourceFunc func(ctx context.Context) ([]Client, error)

func (f clientSourceFunc) ListClients(ctx context.Context) ([]Client, error) {
	return f(ctx)
}
