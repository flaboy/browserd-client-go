package browserd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
)

type SnapshotNode struct {
	ID       string            `json:"id"`
	Tag      string            `json:"tag"`
	Text     string            `json:"text,omitempty"`
	Ref      string            `json:"ref,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
	State    map[string]any    `json:"state,omitempty"`
	Children []*SnapshotNode   `json:"children,omitempty"`
}

type SnapshotOmission struct {
	NodeID string `json:"nodeId"`
	Reason string `json:"reason"`
}

type SnapshotCapture struct {
	Scope     string             `json:"scope"`
	Complete  bool               `json:"complete"`
	Omissions []SnapshotOmission `json:"omissions"`
}

// DecodedSnapshotPage is an in-memory view; SnapshotResult.Page remains verbatim.
type DecodedSnapshotPage struct {
	FormatVersion int
	URL, Title    string
	Groups        map[string]PageTable
	Tree          *SnapshotNode
	Capture       *SnapshotCapture
}

var snapshotNodeID = regexp.MustCompile(`^n[1-9][0-9]*$`)
var snapshotActionRef = regexp.MustCompile(`^e[1-9][0-9]*$`)
var snapshotTag = regexp.MustCompile(`^[a-z][a-z0-9:-]*$`)

func DecodeSnapshotPage(page PageSnapshot) (DecodedSnapshotPage, error) {
	fail := func(reason string) (DecodedSnapshotPage, error) {
		return DecodedSnapshotPage{}, fmt.Errorf("invalid snapshot page: %s", reason)
	}
	raw, err := json.Marshal(page)
	if err != nil {
		return fail("not JSON serializable")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return fail("object required")
	}
	var out DecodedSnapshotPage
	_ = json.Unmarshal(fields["url"], &out.URL)
	_ = json.Unmarshal(fields["title"], &out.Title)
	if version, exists := fields["formatVersion"]; exists {
		if bytes.Equal(version, []byte("null")) || json.Unmarshal(version, &out.FormatVersion) != nil || out.FormatVersion == 0 {
			return fail("invalid formatVersion")
		}
	}
	switch out.FormatVersion {
	case 0, 1:
		for _, key := range []string{"tree", "capture", "encoding", "attributes", "states"} {
			if _, exists := fields[key]; exists {
				return fail("mixed legacy and tree fields")
			}
		}
		if json.Unmarshal(fields["groups"], &out.Groups) != nil || out.Groups == nil {
			return fail("groups object required for legacy format")
		}
		return out, nil
	case 2, 3:
		if _, exists := fields["groups"]; exists {
			return fail("groups cannot accompany a tree")
		}
	default:
		return fail("unsupported formatVersion")
	}
	if out.FormatVersion == 2 {
		for _, key := range []string{"encoding", "attributes", "states"} {
			if _, exists := fields[key]; exists {
				return fail("compact fields require formatVersion 3")
			}
		}
		if json.Unmarshal(fields["tree"], &out.Tree) != nil || out.Tree == nil {
			return fail("tree object required")
		}
	} else {
		var encoding string
		if json.Unmarshal(fields["encoding"], &encoding) != nil || encoding != "dom-tree-tuples-v1" {
			return fail("unsupported tree encoding")
		}
		var attrs []map[string]string
		var states []map[string]any
		if json.Unmarshal(fields["attributes"], &attrs) != nil || attrs == nil || json.Unmarshal(fields["states"], &states) != nil || states == nil {
			return fail("attribute/state dictionaries required")
		}
		for _, entry := range attrs {
			if entry == nil {
				return fail("null attribute entry")
			}
		}
		for _, entry := range states {
			if !validSnapshotState(entry) {
				return fail("invalid state entry")
			}
		}
		count := 0
		var decode func(json.RawMessage, int) (*SnapshotNode, error)
		decode = func(raw json.RawMessage, depth int) (*SnapshotNode, error) {
			count++
			if depth > 160 || count > 12000 {
				return nil, fmt.Errorf("tree exceeds capture limits")
			}
			var cells []json.RawMessage
			if json.Unmarshal(raw, &cells) != nil || len(cells) < 3 {
				return nil, fmt.Errorf("node tuple required")
			}
			node := &SnapshotNode{}
			if json.Unmarshal(cells[0], &node.Tag) != nil || json.Unmarshal(cells[1], &node.ID) != nil {
				return nil, fmt.Errorf("tag and id must be strings")
			}
			if node.Tag == "#text" {
				if len(cells) != 3 || bytes.Equal(cells[2], []byte("null")) || json.Unmarshal(cells[2], &node.Text) != nil {
					return nil, fmt.Errorf("invalid text node")
				}
				return node, nil
			}
			var properties map[string]json.RawMessage
			if json.Unmarshal(cells[2], &properties) != nil || properties == nil {
				return nil, fmt.Errorf("node properties object required")
			}
			for key, value := range properties {
				switch key {
				case "attrs", "state":
					var index int
					if bytes.Equal(value, []byte("null")) || json.Unmarshal(value, &index) != nil || index < 0 {
						return nil, fmt.Errorf("invalid dictionary index")
					}
					if key == "attrs" {
						if index >= len(attrs) {
							return nil, fmt.Errorf("attribute index out of range")
						}
						node.Attrs = attrs[index]
					} else {
						if index >= len(states) {
							return nil, fmt.Errorf("state index out of range")
						}
						node.State = states[index]
					}
				case "ref":
					if json.Unmarshal(value, &node.Ref) != nil || node.Ref == "" {
						return nil, fmt.Errorf("invalid ref")
					}
				default:
					return nil, fmt.Errorf("unsupported node property %q", key)
				}
			}
			for _, child := range cells[3:] {
				decoded, err := decode(child, depth+1)
				if err != nil {
					return nil, err
				}
				node.Children = append(node.Children, decoded)
			}
			return node, nil
		}
		out.Tree, err = decode(fields["tree"], 0)
		if err != nil {
			return fail(err.Error())
		}
	}
	ids, refs := map[string]bool{}, map[string]bool{}
	var validate func(*SnapshotNode, int) error
	validate = func(node *SnapshotNode, depth int) error {
		if node == nil || depth > 160 || len(ids) >= 12000 || !snapshotNodeID.MatchString(node.ID) || ids[node.ID] {
			return fmt.Errorf("invalid/duplicate node id or tree limit")
		}
		ids[node.ID] = true
		if node.Tag == "#text" {
			if len(node.Children) != 0 || node.Ref != "" || node.Attrs != nil || node.State != nil {
				return fmt.Errorf("invalid text node fields")
			}
		} else if !snapshotTag.MatchString(node.Tag) || node.Text != "" {
			return fmt.Errorf("invalid element tag/text")
		}
		if node.Ref != "" {
			if !snapshotActionRef.MatchString(node.Ref) || refs[node.Ref] {
				return fmt.Errorf("invalid/duplicate action ref")
			}
			refs[node.Ref] = true
		}
		if node.State != nil && !validSnapshotState(node.State) {
			return fmt.Errorf("invalid control state")
		}
		for _, child := range node.Children {
			if err := validate(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(out.Tree, 0); err != nil {
		return fail(err.Error())
	}
	if out.Tree.Tag != "html" {
		return fail("document root must be html")
	}
	var capture struct {
		Scope     string             `json:"scope"`
		Complete  *bool              `json:"complete"`
		Omissions []SnapshotOmission `json:"omissions"`
	}
	if json.Unmarshal(fields["capture"], &capture) != nil || capture.Scope != "light-dom" || capture.Complete == nil || capture.Omissions == nil {
		return fail("capture metadata required")
	}
	if *capture.Complete != (len(capture.Omissions) == 0) {
		return fail("capture completeness conflicts with omissions")
	}
	for _, omission := range capture.Omissions {
		if !ids[omission.NodeID] || omission.Reason == "" {
			return fail("invalid omission reference")
		}
	}
	out.Capture = &SnapshotCapture{Scope: capture.Scope, Complete: *capture.Complete, Omissions: capture.Omissions}
	return out, nil
}

func validSnapshotState(state map[string]any) bool {
	if state == nil {
		return false
	}
	for _, value := range state {
		switch v := value.(type) {
		case string, bool:
		case []any:
			for _, item := range v {
				if _, ok := item.(string); !ok {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}
