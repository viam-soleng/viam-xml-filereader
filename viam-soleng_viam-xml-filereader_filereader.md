# Model viam-soleng:viam-xml-filereader:filereader

A Viam sensor that reads an XML file from disk on every call to
`Readings()` and returns its contents as a JSON-compatible object.

## Configuration

```json
{
  "full-file-path": "/data/example.xml"
}
```

### Attributes

| Name              | Type   | Inclusion | Description                                                          |
|-------------------|--------|-----------|----------------------------------------------------------------------|
| `full-file-path`  | string | Required  | Absolute path to the XML file to read on every call to `Readings()`. |
| `recursive-cdata` | bool   | Optional  | Re-parse embedded XML strings (see below). Defaults to `false`.      |

When `recursive-cdata` is `true`, any text-only element value that looks
like XML (starts with `<`, ends with `>`) is fed back through the parser
and replaced with the resulting structured object. This is intended for
the common case where a serialized inner XML document has been stuffed
inside a `<![CDATA[...]]>` block. Strings that fail to parse cleanly are
kept as the original raw string.

### Example Configuration

```json
{
  "name": "xml-reader",
  "model": "viam-soleng:viam-xml-filereader:filereader",
  "type": "sensor",
  "namespace": "rdk",
  "attributes": {
    "full-file-path": "/data/example.xml"
  }
}
```

## Readings

Each call to `Readings()` opens the configured file, parses it as XML, and
returns a single-key map whose key is the document's root element name and
whose value is the recursively converted document.

Mapping rules:

- Child elements become nested objects.
- Repeated children with the same tag become arrays.
- Attributes are emitted with their bare name. If an attribute name
  collides with a same-named child element on the same parent, the
  attribute is prefixed with `-` to keep both.
- Mixed text alongside child elements is preserved under `#text`;
  text-only elements collapse to a plain string.

### Example

Given `/data/example.xml`:

```xml
<catalog>
  <book id="kr-c-1978" edition="1">
    <title>The C Programming Language</title>
    <author>Kernighan</author>
    <author>Ritchie</author>
    <publisher>Prentice Hall</publisher>
    <year>1978</year>
  </book>
</catalog>
```

`Readings()` returns:

```json
{
  "catalog": {
    "book": {
      "id": "kr-c-1978",
      "edition": "1",
      "title": "The C Programming Language",
      "author": ["Kernighan", "Ritchie"],
      "publisher": "Prentice Hall",
      "year": "1978"
    }
  }
}
```
