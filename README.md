# Module viam-xml-filereader

A Viam sensor module that reads an XML file from disk, converts it to a
JSON-compatible object, and returns it via the `Readings()` API.

The module ships prebuilt binaries for the following architectures:

- `linux/amd64`
- `linux/arm64`
- `darwin/arm64`
- `windows/amd64`

## Models

This module provides the following model(s):

- [`viam-soleng:viam-xml-filereader:filereader`](viam-soleng_viam-xml-filereader_filereader.md) — reads an XML file and returns its contents as a JSON object.

## Configuration

Configure a sensor with model `viam-soleng:viam-xml-filereader:filereader`
and supply the absolute path to the XML file to read:

```json
{
  "full-file-path": "/data/example.xml",
  "recursive-cdata": true,
  "upload-on-change-only": true
}
```

### Attributes

| Name | Type | Inclusion | Description |
| --- | --- | --- | --- |
| `full-file-path` | string | Required | Absolute path to the XML file to read on every call to `Readings()`. |
| `recursive-cdata` | bool | Optional | Re-parse embedded XML strings (see below). Defaults to `false`. |
| `upload-on-change-only` | bool | Optional | Skip captures when the file is unchanged (see below). Defaults to `false`. |

When `recursive-cdata` is `true`, any text-only element value that looks
like XML (starts with `<`, ends with `>`) is fed back through the parser
and replaced with the resulting structured object. This is the common
case where a serialized inner XML document has been stuffed inside a
`<![CDATA[...]]>` block. Strings that fail to parse cleanly are kept as
the original raw string.

When `upload-on-change-only` is `true`, each call to `Readings()` computes
the MD5 hash of the file and compares it to the hash from the previous
successful call. If the hash is unchanged, `Readings()` returns
`data.ErrNoCaptureToStore` so the Viam Data Management service skips the
capture. This avoids uploading duplicate snapshots when the underlying
file is polled more frequently than it changes.

## Readings

Each call to `Readings()` opens the configured file, parses it as XML, and
returns a single-key map whose key is the document's root element name and
whose value is the recursively converted document.

XML → JSON mapping rules:

- Element children become nested objects.
- Repeated child elements with the same tag become a JSON array.
- Attributes are emitted with their bare name (e.g. `<item id="5"/>` →
  `{"item": {"id": "5"}}`). If an attribute name collides with a same-named
  child element on the same parent, the attribute is prefixed with `-` to
  keep both — so `<x id="a"><id>b</id></x>` → `{"x": {"-id": "a", "id": "b"}}`.
- Mixed text content alongside child elements is preserved under the key
  `#text`. Text-only elements collapse to a plain string.

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
