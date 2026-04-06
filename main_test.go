package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"regexp"
	"testing"
	"time"
)

func renderPath(t *testing.T, cfg config, input string) string {
	t.Helper()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	makePrintFn(&cfg, w)(input)
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return buf.String()
}

func TestMakePrintFnRelativeAbsoluteBase(t *testing.T) {
	cfg := config{
		base: "/tmp/files-bench",
		left: "/tmp/files-bench",
	}
	got := renderPath(t, cfg, "/tmp/files-bench/a/f1")
	if got != "a/f1\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestMakePrintFnAbsoluteAbsoluteBase(t *testing.T) {
	cfg := config{
		base:     "/tmp/files-bench",
		left:     "/tmp/files-bench",
		absolute: true,
	}
	got := renderPath(t, cfg, "/tmp/files-bench/a/f1")
	if got != "/tmp/files-bench/a/f1\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestMakePrintFnAbsoluteRelativeBase(t *testing.T) {
	cfg := config{
		base:     "repo",
		left:     "/abs/repo",
		absolute: true,
	}
	got := renderPath(t, cfg, "repo/a/f1")
	if got != "/abs/repo/a/f1\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestMakePrintFnDotBase(t *testing.T) {
	cfg := config{
		base: ".",
		left: ".",
	}
	got := renderPath(t, cfg, "./main.go")
	if got != "main.go\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func BenchmarkMakePrintFnRelative(b *testing.B) {
	cfg := config{
		base: "/tmp/files-bench",
		left: "/tmp/files-bench",
	}
	w := bufio.NewWriter(io.Discard)
	printFn := makePrintFn(&cfg, w)
	path := "/tmp/files-bench/deep/tree/file.txt"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printFn(path)
	}
	if err := w.Flush(); err != nil {
		b.Fatalf("flush: %v", err)
	}
}

func BenchmarkMakePrintFnAbsolute(b *testing.B) {
	cfg := config{
		base:     "repo",
		left:     "/abs/repo",
		absolute: true,
	}
	w := bufio.NewWriter(io.Discard)
	printFn := makePrintFn(&cfg, w)
	path := "repo/deep/tree/file.txt"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printFn(path)
	}
	if err := w.Flush(); err != nil {
		b.Fatalf("flush: %v", err)
	}
}

func TestShouldIgnoreDefaultPattern(t *testing.T) {
	cfg := config{fastIgnore: true}
	dir := fakeFileInfo{name: ".git", dir: true}
	file := fakeFileInfo{name: "config", dir: false}

	if !shouldIgnore(&cfg, "/tmp/repo/.git", dir) {
		t.Fatal("expected .git directory to be ignored")
	}
	if shouldIgnore(&cfg, "/tmp/repo/.git/config", file) {
		t.Fatal("did not expect file to be ignored by fast path")
	}
}

func TestShouldIgnoreCustomRegexp(t *testing.T) {
	re := regexpMustCompile(t, `vendor$`)
	cfg := config{ignorere: re}
	dir := fakeFileInfo{name: "vendor", dir: true}

	if !shouldIgnore(&cfg, "/tmp/repo/vendor", dir) {
		t.Fatal("expected vendor directory to be ignored")
	}
}

type fakeFileInfo struct {
	name string
	dir  bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.dir }
func (f fakeFileInfo) Sys() any           { return nil }

func regexpMustCompile(t *testing.T, expr string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(expr)
	if err != nil {
		t.Fatalf("compile regexp: %v", err)
	}
	return re
}
