package browserd

import (
	"regexp"
	"sort"
	"strings"
)

const DefaultPageToolBridgeName = "__browserdPageToolCall"

var pageToolBridgeNamePattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

var supportedPageToolMethods = map[string]struct{}{
	"page.url":              {},
	"page.title":            {},
	"page.viewport":         {},
	"page.waitForLoadState": {},
	"page.waitForTimeout":   {},
	"keyboard.type":         {},
	"keyboard.press":        {},
	"keyboard.down":         {},
	"keyboard.up":           {},
	"keyboard.insertText":   {},
	"mouse.move":            {},
	"mouse.click":           {},
	"mouse.dblclick":        {},
	"mouse.down":            {},
	"mouse.up":              {},
	"mouse.wheel":           {},
	"element.click":         {},
	"element.hover":         {},
	"element.type":          {},
	"element.fill":          {},
	"element.press":         {},
	"element.text":          {},
	"element.html":          {},
	"element.box":           {},
	"element.exists":        {},
}

func SupportedPageToolMethods() []string {
	methods := make([]string, 0, len(supportedPageToolMethods))
	for method := range supportedPageToolMethods {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func IsSupportedPageToolMethod(method string) bool {
	_, ok := supportedPageToolMethods[strings.TrimSpace(method)]
	return ok
}

func BuildPageToolRuntimeScript(bridgeName string) (string, error) {
	name := strings.TrimSpace(bridgeName)
	if name == "" {
		name = DefaultPageToolBridgeName
	}
	if !pageToolBridgeNamePattern.MatchString(name) {
		return "", Error{Code: "browserd_pagetool_bridge_name_invalid", Message: "pageTool bridge name is invalid"}
	}
	call := "__browserdPageToolCall"
	var b strings.Builder
	b.WriteString("const ")
	b.WriteString(call)
	b.WriteString(" = async (method, payload) => {\n")
	b.WriteString("  const fn = window[")
	b.WriteString(quoteJSString(name))
	b.WriteString("];\n")
	b.WriteString("  if (!fn || typeof fn !== \"function\") {\n")
	b.WriteString("    throw new Error(\"BROWSERD_PAGETOOL_BRIDGE_UNAVAILABLE: pageTool bridge is unavailable in this browser runtime\");\n")
	b.WriteString("  }\n")
	b.WriteString("  return await fn({ method, payload: payload || {} });\n")
	b.WriteString("};\n")
	b.WriteString("const pageTool = {\n")
	b.WriteString("  page: {\n")
	b.WriteString("    url: () => ")
	b.WriteString(call)
	b.WriteString("(\"page.url\", {}),\n")
	b.WriteString("    title: () => ")
	b.WriteString(call)
	b.WriteString("(\"page.title\", {}),\n")
	b.WriteString("    viewport: () => ")
	b.WriteString(call)
	b.WriteString("(\"page.viewport\", {}),\n")
	b.WriteString("    waitForLoadState: (state, options) => ")
	b.WriteString(call)
	b.WriteString("(\"page.waitForLoadState\", { state, options }),\n")
	b.WriteString("    waitForTimeout: (ms) => ")
	b.WriteString(call)
	b.WriteString("(\"page.waitForTimeout\", { ms })\n")
	b.WriteString("  },\n")
	b.WriteString("  keyboard: {\n")
	b.WriteString("    type: (text, options) => ")
	b.WriteString(call)
	b.WriteString("(\"keyboard.type\", { text, options }),\n")
	b.WriteString("    press: (key, options) => ")
	b.WriteString(call)
	b.WriteString("(\"keyboard.press\", { key, options }),\n")
	b.WriteString("    down: (key, options) => ")
	b.WriteString(call)
	b.WriteString("(\"keyboard.down\", { key, options }),\n")
	b.WriteString("    up: (key, options) => ")
	b.WriteString(call)
	b.WriteString("(\"keyboard.up\", { key, options }),\n")
	b.WriteString("    insertText: (text, options) => ")
	b.WriteString(call)
	b.WriteString("(\"keyboard.insertText\", { text, options })\n")
	b.WriteString("  },\n")
	b.WriteString("  mouse: {\n")
	b.WriteString("    move: (x, y, options) => ")
	b.WriteString(call)
	b.WriteString("(\"mouse.move\", { x, y, options }),\n")
	b.WriteString("    click: (x, y, options) => ")
	b.WriteString(call)
	b.WriteString("(\"mouse.click\", { x, y, options }),\n")
	b.WriteString("    dblclick: (x, y, options) => ")
	b.WriteString(call)
	b.WriteString("(\"mouse.dblclick\", { x, y, options }),\n")
	b.WriteString("    down: (options) => ")
	b.WriteString(call)
	b.WriteString("(\"mouse.down\", { options }),\n")
	b.WriteString("    up: (options) => ")
	b.WriteString(call)
	b.WriteString("(\"mouse.up\", { options }),\n")
	b.WriteString("    wheel: (deltaX, deltaY, options) => ")
	b.WriteString(call)
	b.WriteString("(\"mouse.wheel\", { deltaX, deltaY, options })\n")
	b.WriteString("  },\n")
	b.WriteString("  element: {\n")
	b.WriteString("    click: (target, options) => ")
	b.WriteString(call)
	b.WriteString("(\"element.click\", { target, options }),\n")
	b.WriteString("    hover: (target, options) => ")
	b.WriteString(call)
	b.WriteString("(\"element.hover\", { target, options }),\n")
	b.WriteString("    type: (target, text, options) => ")
	b.WriteString(call)
	b.WriteString("(\"element.type\", { target, text, options }),\n")
	b.WriteString("    fill: (target, value, options) => ")
	b.WriteString(call)
	b.WriteString("(\"element.fill\", { target, value, options }),\n")
	b.WriteString("    press: (target, key, options) => ")
	b.WriteString(call)
	b.WriteString("(\"element.press\", { target, key, options }),\n")
	b.WriteString("    text: (target, options) => ")
	b.WriteString(call)
	b.WriteString("(\"element.text\", { target, options }),\n")
	b.WriteString("    html: (target, options) => ")
	b.WriteString(call)
	b.WriteString("(\"element.html\", { target, options }),\n")
	b.WriteString("    box: (target, options) => ")
	b.WriteString(call)
	b.WriteString("(\"element.box\", { target, options }),\n")
	b.WriteString("    exists: (target, options) => ")
	b.WriteString(call)
	b.WriteString("(\"element.exists\", { target, options })\n")
	b.WriteString("  }\n")
	b.WriteString("};")
	return b.String(), nil
}

func quoteJSString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
