package transpiler

import (
	"strings"
	"testing"
)

func TestGenerateGoModContentWithoutReplace(t *testing.T) {
	content := generateGoModContent("hello", defaultGoblinRuntimeVersion, "")

	if !strings.Contains(content, "module hello\n") {
		t.Fatalf("missing module declaration:\n%s", content)
	}
	if !strings.Contains(content, "go 1.19\n") {
		t.Fatalf("missing go version:\n%s", content)
	}
	if !strings.Contains(content, "require github.com/aisk/goblin "+defaultGoblinRuntimeVersion+"\n") {
		t.Fatalf("missing runtime require with expected version:\n%s", content)
	}
	if strings.Contains(content, "\nreplace github.com/aisk/goblin => ") {
		t.Fatalf("unexpected replace directive:\n%s", content)
	}
}

func TestGenerateGoModContentWithReplace(t *testing.T) {
	content := generateGoModContent("hello", defaultGoblinRuntimeVersion, "/tmp/goblin")

	if !strings.Contains(content, "require github.com/aisk/goblin "+defaultGoblinRuntimeVersion+"\n") {
		t.Fatalf("missing runtime require with expected version:\n%s", content)
	}
	if !strings.Contains(content, "replace github.com/aisk/goblin => /tmp/goblin\n") {
		t.Fatalf("missing replace directive:\n%s", content)
	}
}

func TestGenerateGoModContentDoesNotUsePlaceholderVersion(t *testing.T) {
	content := generateGoModContent("hello", defaultGoblinRuntimeVersion, "")

	if strings.Contains(content, "v0.0.0-00010101000000-000000000000") {
		t.Fatalf("placeholder version should not appear:\n%s", content)
	}
}
