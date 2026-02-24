package api

import "testing"

func TestShouldRegisterTaskTool(t *testing.T) {
	if !shouldRegisterTaskTool(EntryPointCLI) {
		t.Fatal("cli entrypoint should register task tool")
	}
	if shouldRegisterTaskTool(EntryPointCI) {
		t.Fatal("ci should not register task tool")
	}
}
