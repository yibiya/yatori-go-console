package bootstrap

import (
	"reflect"
	"testing"
)

func TestRunWithHooksExecutesStartupStepsInOrder(t *testing.T) {
	var calls []string

	runWithHooks(hooks{
		initConsole: func() {
			calls = append(calls, "init")
		},
		printLogo: func() {
			calls = append(calls, "logo")
		},
		showAnnouncement: func() {
			calls = append(calls, "announcement")
		},
		launch: func() {
			calls = append(calls, "launch")
		},
	})

	want := []string{"init", "logo", "announcement", "launch"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected startup order: got %v want %v", calls, want)
	}
}
