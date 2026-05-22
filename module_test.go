package viamxmlfilereader

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeXML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.xml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func readingsFor(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	return readingsForCfg(t, &Config{FullFilePath: writeXML(t, body)})
}

func readingsForCfg(t *testing.T, cfg *Config) map[string]interface{} {
	t.Helper()
	s := &viamXmlFilereaderFilereader{cfg: cfg}
	out, err := s.Readings(context.Background(), nil)
	if err != nil {
		t.Fatalf("Readings: %v", err)
	}
	return out
}

func TestReadingsSimple(t *testing.T) {
	got := readingsFor(t, `<root><a>1</a><b>two</b></root>`)
	want := map[string]interface{}{
		"root": map[string]interface{}{
			"a": "1",
			"b": "two",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestReadingsAttributesAndRepeated(t *testing.T) {
	got := readingsFor(t, `<root id="x"><item>a</item><item>b</item></root>`)
	root := got["root"].(map[string]interface{})
	if root["id"] != "x" {
		t.Fatalf("attr: %#v (full root: %#v)", root["id"], root)
	}
	if _, prefixed := root["-id"]; prefixed {
		t.Fatalf("attribute should not be prefixed when no collision: %#v", root)
	}
	items, ok := root["item"].([]interface{})
	if !ok || len(items) != 2 || items[0] != "a" || items[1] != "b" {
		t.Fatalf("items: %#v", root["item"])
	}
}

func TestAttributeCollisionPrefixed(t *testing.T) {
	got := readingsFor(t, `<root id="attr-value"><id>child-value</id></root>`)
	root := got["root"].(map[string]interface{})
	if root["id"] != "child-value" {
		t.Fatalf("expected child element to keep bare name: %#v", root)
	}
	if root["-id"] != "attr-value" {
		t.Fatalf("expected colliding attribute to be prefixed: %#v", root)
	}
}

func TestRecursiveCDATAOff(t *testing.T) {
	body := `<root><layout><![CDATA[<Inner foo="1"><Child>hi</Child></Inner>]]></layout></root>`
	got := readingsFor(t, body)
	layout := got["root"].(map[string]interface{})["layout"]
	if s, ok := layout.(string); !ok || !strings.Contains(s, "<Inner") {
		t.Fatalf("expected raw string, got %#v", layout)
	}
}

func TestRecursiveCDATAOn(t *testing.T) {
	body := `<root><layout><![CDATA[<Inner foo="1"><Child>hi</Child></Inner>]]></layout></root>`
	path := writeXML(t, body)
	got := readingsForCfg(t, &Config{FullFilePath: path, RecursiveCDATA: true})
	layout, ok := got["root"].(map[string]interface{})["layout"].(map[string]interface{})
	if !ok {
		t.Fatalf("layout not re-parsed: %#v", got)
	}
	inner, ok := layout["Inner"].(map[string]interface{})
	if !ok {
		t.Fatalf("Inner missing: %#v", layout)
	}
	if inner["foo"] != "1" || inner["Child"] != "hi" {
		t.Fatalf("inner contents wrong: %#v", inner)
	}
}

func TestRecursiveCDATAFallsBackOnGarbage(t *testing.T) {
	body := `<root><blob><![CDATA[<not-real-xml-because-it-doesnt-close]]></blob></root>`
	path := writeXML(t, body)
	got := readingsForCfg(t, &Config{FullFilePath: path, RecursiveCDATA: true})
	if _, ok := got["root"].(map[string]interface{})["blob"].(string); !ok {
		t.Fatalf("expected raw string fallback, got %#v", got["root"])
	}
}

func TestValidateRequiresPath(t *testing.T) {
	if _, _, err := (&Config{}).Validate("components.0"); err == nil {
		t.Fatal("expected error for empty full-file-path")
	}
	if _, _, err := (&Config{FullFilePath: "/tmp/x.xml"}).Validate("components.0"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
