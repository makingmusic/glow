package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var glowBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "glow-e2e-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	glowBin = filepath.Join(tmp, "glow-test")
	cmd := exec.Command("go", "build", "-o", glowBin, "..")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build glow: " + err.Error())
	}

	os.Exit(m.Run())
}

func TestRenderMarkdownFile(t *testing.T) {
	out, err := exec.Command(glowBin, "testdata/test.md").CombinedOutput()
	if err != nil {
		t.Fatalf("glow testdata/test.md failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestRenderWithStyle(t *testing.T) {
	out, err := exec.Command(glowBin, "-s", "dark", "testdata/test.md").CombinedOutput()
	if err != nil {
		t.Fatalf("glow -s dark failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestRenderWithWidth(t *testing.T) {
	out, err := exec.Command(glowBin, "-w", "40", "testdata/test.md").CombinedOutput()
	if err != nil {
		t.Fatalf("glow -w 40 failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestRenderWithLineNumbers(t *testing.T) {
	out, err := exec.Command(glowBin, "-l", "testdata/test.md").CombinedOutput()
	if err != nil {
		t.Fatalf("glow -l failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestStdinPipe(t *testing.T) {
	cmd := exec.Command(glowBin)
	cmd.Stdin = strings.NewReader("# Hello\n\nWorld")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("echo | glow failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Hello") {
		t.Errorf("expected output to contain 'Hello', got: %s", out)
	}
}

func TestInvalidFile(t *testing.T) {
	err := exec.Command(glowBin, "nonexistent.md").Run()
	if err == nil {
		t.Error("expected non-zero exit for nonexistent file")
	}
}

func TestLongFilePipedPrintsOutput(t *testing.T) {
	// When stdout is not a terminal (piped via exec), long files should
	// still be printed to stdout rather than opening the TUI pager.
	out, err := exec.Command(glowBin, "testdata/long.md").CombinedOutput()
	if err != nil {
		t.Fatalf("glow testdata/long.md failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Long Test Document") {
		t.Errorf("expected output to contain title, got: %s", output)
	}
	if !strings.Contains(output, "The end.") {
		t.Errorf("expected output to contain last line, got: %s", output)
	}
}

func TestLongStdinPipedPrintsOutput(t *testing.T) {
	// Long content via stdin with piped stdout should print, not open TUI.
	lines := "# Big Document\n\n" + strings.Repeat("Line of text.\n\n", 100) + "Final line.\n"
	cmd := exec.Command(glowBin)
	cmd.Stdin = strings.NewReader(lines)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("echo | glow failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Big Document") {
		t.Errorf("expected output to contain title, got: %s", output)
	}
	if !strings.Contains(output, "Final line.") {
		t.Errorf("expected output to contain last line, got: %s", output)
	}
}

func TestHelpFlag(t *testing.T) {
	out, err := exec.Command(glowBin, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("glow --help failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(strings.ToLower(output), "glow") {
		t.Errorf("expected help output to contain 'glow', got: %s", output)
	}
}
