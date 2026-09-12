package browserd

import (
	"encoding/json"
	"strings"
	"testing"
)

const compactPageFixture = `{"formatVersion":3,"encoding":"dom-tree-tuples-v1","url":"https://example.com/","title":"Tree","tree":["html","n1",{},["body","n2",{},["a","n3",{"attrs":0,"ref":"e1"},["#text","n4","Entry A"]],["img","n5",{"attrs":1}],["input","n6",{"state":0,"ref":"e2"}]]],"attributes":[{"href":"https://example.com/a","class":"shared","aria-labelledby":"label"},{"src":"https://example.com/a.png","alt":"A"}],"states":[{"checked":false,"selected":["a","b"]}],"capture":{"scope":"light-dom","complete":true,"omissions":[]}}`

func TestCompactSnapshotAcceptedWithoutChangingRawPage(t *testing.T) {
	var page PageSnapshot
	if err := json.Unmarshal([]byte(compactPageFixture), &page); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(page)
	if err := ValidateSnapshotResult(&SnapshotResult{SnapshotID: "s", Page: page}); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(page)
	if string(before) != string(after) {
		t.Fatal("validation mutated the canonical observation")
	}
}

func TestCompactSnapshotRejectsInvalidStructure(t *testing.T) {
	for _, raw := range []string{
		strings.Replace(compactPageFixture, `"attrs":0`, `"attrs":99`, 1),
		strings.Replace(compactPageFixture, `"attrs":0`, `"attrs":0.5`, 1),
		strings.Replace(compactPageFixture, `"n4"`, `"n3"`, 1),
		strings.Replace(compactPageFixture, `"e2"`, `"e1"`, 1),
		strings.Replace(compactPageFixture, `"Entry A"`, `42`, 1),
		strings.Replace(compactPageFixture, `"formatVersion":3`, `"formatVersion":99`, 1),
		strings.Replace(compactPageFixture, `"formatVersion":3`, `"formatVersion":null`, 1),
		strings.Replace(compactPageFixture, `"formatVersion":3`, `"groups":{},"formatVersion":3`, 1),
		strings.Replace(compactPageFixture, `"dom-tree-tuples-v1"`, `"unknown"`, 1),
		strings.Replace(compactPageFixture, `"complete":true`, `"complete":null`, 1),
		strings.Replace(compactPageFixture, `"omissions":[]`, `"omissions":[{"nodeId":"n99","reason":"frame"}]`, 1),
	} {
		var page PageSnapshot
		if err := json.Unmarshal([]byte(raw), &page); err != nil {
			t.Fatal(err)
		}
		if err := ValidateSnapshotResult(&SnapshotResult{SnapshotID: "s", Page: page}); err == nil {
			t.Fatalf("accepted invalid or mixed structure: %s", raw)
		}
	}
}

func TestSnapshotDecoderSelectsLegacyAndObjectTreeByVersion(t *testing.T) {
	for _, fixture := range []struct {
		raw     string
		version int
	}{
		{`{"url":"https://example.com/","groups":{"texts":{"columns":["text"],"rows":[["Legacy"]]}}}`, 0},
		{`{"formatVersion":1,"url":"https://example.com/","groups":{}}`, 1},
		{`{"formatVersion":2,"url":"https://example.com/","tree":{"id":"n1","tag":"html","children":[{"id":"n2","tag":"input","ref":"e1","attrs":{"type":"checkbox"},"state":{"checked":false}}]},"capture":{"scope":"light-dom","complete":true,"omissions":[]}}`, 2},
	} {
		var page PageSnapshot
		if err := json.Unmarshal([]byte(fixture.raw), &page); err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(page)
		doc, err := DecodeSnapshotPage(page)
		if err != nil || doc.FormatVersion != fixture.version {
			t.Fatalf("version %d: %+v %v", fixture.version, doc, err)
		}
		after, _ := json.Marshal(page)
		if string(before) != string(after) {
			t.Fatal("decoding changed raw page")
		}
		if fixture.version == 2 && doc.Tree.Children[0].State["checked"] != false {
			t.Fatal("false control state lost")
		}
	}
}
