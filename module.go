package viamxmlfilereader

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"

	sensor "go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var Filereader = resource.NewModel("viam-soleng", "viam-xml-filereader", "filereader")

func init() {
	resource.RegisterComponent(sensor.API, Filereader,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: newViamXmlFilereaderFilereader,
		},
	)
}

type Config struct {
	FullFilePath string `json:"full-file-path"`
	// RecursiveCDATA, when true, causes text-only element values that look
	// like XML (start with '<', end with '>') to be re-parsed as XML and
	// replaced with the resulting structured object. Used to flatten the
	// common pattern of stuffing a serialized inner XML document inside a
	// CDATA section. Strings that fail to parse cleanly are kept as-is.
	RecursiveCDATA bool `json:"recursive-cdata,omitempty"`
}

func (cfg *Config) Validate(path string) ([]string, []string, error) {
	if strings.TrimSpace(cfg.FullFilePath) == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "full-file-path")
	}
	return nil, nil, nil
}

type viamXmlFilereaderFilereader struct {
	resource.AlwaysRebuild
	resource.Named

	name   resource.Name
	logger logging.Logger
	cfg    *Config

	cancelCtx  context.Context
	cancelFunc func()
}

func newViamXmlFilereaderFilereader(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}
	return NewFilereader(ctx, deps, rawConf.ResourceName(), conf, logger)
}

func NewFilereader(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *Config, logger logging.Logger) (sensor.Sensor, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	s := &viamXmlFilereaderFilereader{
		name:       name,
		logger:     logger,
		cfg:        conf,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	return s, nil
}

func (s *viamXmlFilereaderFilereader) Name() resource.Name {
	return s.name
}

type parseOpts struct {
	recursiveCDATA bool
}

func (s *viamXmlFilereaderFilereader) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	path := s.cfg.FullFilePath
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open XML file %q: %w", path, err)
	}
	defer f.Close()

	opts := parseOpts{recursiveCDATA: s.cfg.RecursiveCDATA}
	decoder := xml.NewDecoder(f)
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return map[string]interface{}{}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse XML: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		child, err := parseElement(decoder, start, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to parse XML element %q: %w", start.Name.Local, err)
		}
		return map[string]interface{}{start.Name.Local: child}, nil
	}
}

// parseElement walks the decoder for a single element and returns either a
// string (text-only element) or a map[string]interface{} (element with
// attributes and/or child elements). Attribute keys use the bare attribute
// name unless they would collide with a same-named child element on the
// same parent, in which case the attribute is prefixed with "-". Mixed
// text content is stored under "#text".
//
// When opts.recursiveCDATA is set, a text-only value that looks like XML is
// fed back through the parser; on a clean parse the structured result is
// returned instead of the raw string.
func parseElement(decoder *xml.Decoder, start xml.StartElement, opts parseOpts) (interface{}, error) {
	result := make(map[string]interface{})
	childNames := make(map[string]bool)

	var textBuf strings.Builder
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			key := t.Name.Local
			childNames[key] = true
			child, err := parseElement(decoder, t, opts)
			if err != nil {
				return nil, err
			}
			if existing, ok := result[key]; ok {
				if arr, isArr := existing.([]interface{}); isArr {
					result[key] = append(arr, child)
				} else {
					result[key] = []interface{}{existing, child}
				}
			} else {
				result[key] = child
			}
		case xml.CharData:
			textBuf.Write(t)
		case xml.EndElement:
			for _, attr := range start.Attr {
				name := attr.Name.Local
				if childNames[name] {
					name = "-" + name
				}
				result[name] = attr.Value
			}
			text := strings.TrimSpace(textBuf.String())
			if len(result) == 0 {
				if opts.recursiveCDATA && looksLikeXML(text) {
					if parsed, ok := tryParseEmbeddedXML(text, opts); ok {
						return parsed, nil
					}
				}
				return text, nil
			}
			if text != "" {
				result["#text"] = text
			}
			return result, nil
		}
	}
}

func looksLikeXML(s string) bool {
	return strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">")
}

// tryParseEmbeddedXML attempts to parse s as a standalone XML document.
// On a clean parse (one root element, only whitespace after) it returns the
// converted structure; otherwise it returns ok=false so the caller falls
// back to the raw string.
func tryParseEmbeddedXML(s string, opts parseOpts) (interface{}, bool) {
	dec := xml.NewDecoder(strings.NewReader(s))
	var converted interface{}
	var rootName string
	gotRoot := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if gotRoot {
				return nil, false
			}
			child, err := parseElement(dec, t, opts)
			if err != nil {
				return nil, false
			}
			rootName = t.Name.Local
			converted = child
			gotRoot = true
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, false
			}
		}
	}
	if !gotRoot {
		return nil, false
	}
	return map[string]interface{}{rootName: converted}, true
}

func (s *viamXmlFilereaderFilereader) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *viamXmlFilereaderFilereader) Status(ctx context.Context) (map[string]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *viamXmlFilereaderFilereader) Close(context.Context) error {
	s.cancelFunc()
	return nil
}
