package browserd

import (
	"strings"
	"testing"
)

func TestSupportedPageToolMethodsAreBrowserNativeOnly(t *testing.T) {
	if !IsSupportedPageToolMethod("element.click") {
		t.Fatal("element.click must be supported")
	}
	if !IsSupportedPageToolMethod("mouse.wheel") {
		t.Fatal("mouse.wheel must be supported")
	}
	if IsSupportedPageToolMethod("visual.getViewportContent") {
		t.Fatal("visual methods must not be production-supported without a browserd visual provider")
	}
}

func TestBuildPageToolRuntimeScriptUsesConfiguredBridge(t *testing.T) {
	script, err := BuildPageToolRuntimeScript("__browserdPageToolCall")
	if err != nil {
		t.Fatalf("BuildPageToolRuntimeScript: %v", err)
	}
	for _, want := range []string{
		"const pageTool =",
		`__browserdPageToolCall("page.title", {})`,
		`__browserdPageToolCall("element.click"`,
		`__browserdPageToolCall("mouse.wheel"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("runtime script missing %s:\n%s", want, script)
		}
	}
	if strings.Contains(script, "visual.getViewportContent") {
		t.Fatal("runtime script must not expose unsupported visual methods")
	}
}

func TestValidatePageToolInput(t *testing.T) {
	if err := ValidatePageToolInput(PageToolInput{Method: "page.title"}); err != nil {
		t.Fatalf("page.title should be valid: %v", err)
	}
	if err := ValidatePageToolInput(PageToolInput{Method: "visual.getViewportContent"}); err == nil {
		t.Fatal("unsupported visual method should fail validation")
	}
	if err := ValidatePageToolInput(PageToolInput{Method: ""}); err == nil {
		t.Fatal("empty method should fail validation")
	}
}
