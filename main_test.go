package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apex/log"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
	return path
}

func TestDataFormat_String(t *testing.T) {
	cases := map[DataFormat]string{
		FormatYAML:    "YAML",
		FormatTOML:    "TOML",
		FormatJSON:    "JSON",
		DataFormat(9): "Unknown",
	}
	for df, want := range cases {
		if got := df.String(); got != want {
			t.Errorf("DataFormat(%d).String() = %q, want %q", df, got, want)
		}
	}
}

func TestDataFormat_Parse(t *testing.T) {
	tests := []struct {
		name    string
		format  DataFormat
		input   string
		wantKey string
		wantVal any
	}{
		{"yaml", FormatYAML, "foo: bar\n", "foo", "bar"},
		{"toml", FormatTOML, "foo = \"bar\"\n", "foo", "bar"},
		{"json", FormatJSON, `{"foo":"bar"}`, "foo", "bar"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := make(Data)
			if _, err := tc.format.Parse(strings.NewReader(tc.input), &data); err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			if data[tc.wantKey] != tc.wantVal {
				t.Errorf("data[%q] = %v, want %v", tc.wantKey, data[tc.wantKey], tc.wantVal)
			}
		})
	}
}

func TestDataFormat_Parse_Errors(t *testing.T) {
	t.Run("unsupported format", func(t *testing.T) {
		data := make(Data)
		if _, err := DataFormat(99).Parse(strings.NewReader("anything"), &data); err == nil {
			t.Fatal("expected error for unsupported format, got nil")
		}
	})
	t.Run("malformed yaml", func(t *testing.T) {
		data := make(Data)
		if _, err := FormatYAML.Parse(strings.NewReader("foo: [unterminated"), &data); err == nil {
			t.Fatal("expected error for malformed yaml, got nil")
		}
	})
	t.Run("malformed json", func(t *testing.T) {
		data := make(Data)
		if _, err := FormatJSON.Parse(strings.NewReader("{not json"), &data); err == nil {
			t.Fatal("expected error for malformed json, got nil")
		}
	})
	t.Run("malformed toml", func(t *testing.T) {
		data := make(Data)
		if _, err := FormatTOML.Parse(strings.NewReader("== not toml"), &data); err == nil {
			t.Fatal("expected error for malformed toml, got nil")
		}
	})
}

func TestDataFile_Set_ExtensionInference(t *testing.T) {
	cases := []struct {
		input      string
		wantFormat DataFormat
	}{
		{"foo.yaml", FormatYAML},
		{"foo.yml", FormatYAML},
		{"foo.toml", FormatTOML},
		{"foo.json", FormatJSON},
		{"path/to/file.yaml", FormatYAML},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			var df DataFile
			if err := df.Set(tc.input); err != nil {
				t.Fatalf("Set(%q) error: %v", tc.input, err)
			}
			if df.DataFormat != tc.wantFormat {
				t.Errorf("DataFormat = %v, want %v", df.DataFormat, tc.wantFormat)
			}
			if df.Filename != tc.input {
				t.Errorf("Filename = %q, want %q", df.Filename, tc.input)
			}
		})
	}
}

func TestDataFile_Set_ExplicitFormat(t *testing.T) {
	cases := []struct {
		input      string
		wantName   string
		wantFormat DataFormat
	}{
		{"data,yaml", "data", FormatYAML},
		{"data,toml", "data", FormatTOML},
		{"data,json", "data", FormatJSON},
		{"weird-name.txt,yaml", "weird-name.txt", FormatYAML},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			var df DataFile
			if err := df.Set(tc.input); err != nil {
				t.Fatalf("Set(%q) error: %v", tc.input, err)
			}
			if df.Filename != tc.wantName {
				t.Errorf("Filename = %q, want %q", df.Filename, tc.wantName)
			}
			if df.DataFormat != tc.wantFormat {
				t.Errorf("DataFormat = %v, want %v", df.DataFormat, tc.wantFormat)
			}
		})
	}
}

func TestDataFile_Set_Errors(t *testing.T) {
	cases := []string{
		"foo.xml",
		"noextension",
		"foo,bogus",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			var df DataFile
			if err := df.Set(input); err == nil {
				t.Errorf("Set(%q) = nil, want error", input)
			}
		})
	}
}

func TestDataFile_String(t *testing.T) {
	df := &DataFile{DataFormat: FormatYAML, Filename: "foo.yaml"}
	if got := df.String(); got != "foo.yaml (YAML)" {
		t.Errorf("String() = %q", got)
	}
}

func TestDataFile_Type(t *testing.T) {
	var df DataFile
	if got := df.Type(); got != "datafile" {
		t.Errorf("Type() = %q, want %q", got, "datafile")
	}
}

func newOptions(files ...DataFile) *Options {
	return &Options{
		DataFiles: files,
		Interface: log.Log,
	}
}

func TestOptions_Data_OverrideOrder(t *testing.T) {
	dir := t.TempDir()
	yamlPath := writeFile(t, dir, "a.yaml", "format: yaml\nshared: from-yaml\ncontact:\n  name: John\n")
	tomlPath := writeFile(t, dir, "b.toml", "format = \"toml\"\nshared = \"from-toml\"\n[database]\nserver = \"localhost\"\n")
	jsonPath := writeFile(t, dir, "c.json", `{"format":"json","version":1}`)

	opts := newOptions(
		DataFile{DataFormat: FormatYAML, Filename: yamlPath},
		DataFile{DataFormat: FormatTOML, Filename: tomlPath},
		DataFile{DataFormat: FormatJSON, Filename: jsonPath},
	)

	data, err := opts.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}

	if data["format"] != "json" {
		t.Errorf("format = %v, want %q (last file should win)", data["format"], "json")
	}
	if data["shared"] != "from-toml" {
		t.Errorf("shared = %v, want %q (toml overrides yaml, json doesn't touch it)", data["shared"], "from-toml")
	}
	if _, ok := data["contact"]; !ok {
		t.Errorf("contact missing: keys from earlier files must be preserved when not overridden")
	}
	if _, ok := data["database"]; !ok {
		t.Errorf("database missing: keys from middle file must be preserved")
	}
	// JSON numbers decode to float64
	if v, ok := data["version"].(float64); !ok || v != 1 {
		t.Errorf("version = %v (%T), want float64(1)", data["version"], data["version"])
	}
}

func TestOptions_Data_ReverseOrder(t *testing.T) {
	// Same files, reverse order — verifies override is positional, not format-based.
	dir := t.TempDir()
	yamlPath := writeFile(t, dir, "a.yaml", "format: yaml\n")
	jsonPath := writeFile(t, dir, "c.json", `{"format":"json"}`)

	opts := newOptions(
		DataFile{DataFormat: FormatJSON, Filename: jsonPath},
		DataFile{DataFormat: FormatYAML, Filename: yamlPath},
	)
	data, err := opts.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if data["format"] != "yaml" {
		t.Errorf("format = %v, want %q (last file wins regardless of format)", data["format"], "yaml")
	}
}

func TestOptions_Data_NoFiles(t *testing.T) {
	opts := newOptions()
	data, err := opts.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data, got %v", data)
	}
}

func TestOptions_Data_MissingFile(t *testing.T) {
	opts := newOptions(DataFile{DataFormat: FormatYAML, Filename: "/nonexistent/path/to/file.yaml"})
	if _, err := opts.Data(); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestOptions_Data_MalformedFile(t *testing.T) {
	dir := t.TempDir()
	bad := writeFile(t, dir, "bad.json", "{not json")
	opts := newOptions(DataFile{DataFormat: FormatJSON, Filename: bad})
	if _, err := opts.Data(); err == nil {
		t.Fatal("expected error for malformed file, got nil")
	}
}

func TestOptions_Template(t *testing.T) {
	dir := t.TempDir()
	tmplPath := writeFile(t, dir, "t.txt", "Hello {{.name | upper}}!")
	opts := &Options{TemplateFile: tmplPath, Interface: log.Log}

	tmpl, err := opts.Template()
	if err != nil {
		t.Fatalf("Template() error: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, Data{"name": "world"}); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if got := buf.String(); got != "Hello WORLD!" {
		t.Errorf("output = %q, want %q (sprig 'upper' func must be wired in)", got, "Hello WORLD!")
	}
}

func TestOptions_Template_Errors(t *testing.T) {
	t.Run("empty filename", func(t *testing.T) {
		opts := &Options{Interface: log.Log}
		if _, err := opts.Template(); err == nil {
			t.Fatal("expected error for empty template file, got nil")
		}
	})
	t.Run("missing file", func(t *testing.T) {
		opts := &Options{TemplateFile: "/nonexistent/template.txt", Interface: log.Log}
		if _, err := opts.Template(); err == nil {
			t.Fatal("expected error for missing template, got nil")
		}
	})
	t.Run("malformed template", func(t *testing.T) {
		dir := t.TempDir()
		bad := writeFile(t, dir, "bad.tmpl", "{{ .unterminated ")
		opts := &Options{TemplateFile: bad, Interface: log.Log}
		if _, err := opts.Template(); err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})
}

func TestOptions_OutStream_Stdout(t *testing.T) {
	empty := ""
	opts := &Options{Outfile: &empty, Interface: log.Log}
	stream, err := opts.OutStream()
	if err != nil {
		t.Fatalf("OutStream() error: %v", err)
	}
	if stream != os.Stdout {
		t.Errorf("expected os.Stdout when outfile is empty")
	}

	// nil outfile pointer also yields stdout
	opts2 := &Options{Outfile: nil, Interface: log.Log}
	stream2, err := opts2.OutStream()
	if err != nil {
		t.Fatalf("OutStream() with nil outfile error: %v", err)
	}
	if stream2 != os.Stdout {
		t.Errorf("expected os.Stdout when outfile is nil")
	}
}

func TestOptions_OutStream_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	opts := &Options{Outfile: &path, Interface: log.Log}

	stream, err := opts.OutStream()
	if err != nil {
		t.Fatalf("OutStream() error: %v", err)
	}
	if _, err := io.WriteString(stream, "hello"); err != nil {
		t.Fatalf("write error: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back error: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("file contents = %q, want %q", got, "hello")
	}
}

func TestOptions_OutStream_FileError(t *testing.T) {
	bad := "/nonexistent/directory/out.txt"
	opts := &Options{Outfile: &bad, Interface: log.Log}
	if _, err := opts.OutStream(); err == nil {
		t.Fatal("expected error creating file in nonexistent directory, got nil")
	}
}

func TestValue_Set_StringValue(t *testing.T) {
	var v Value
	if err := v.Set("name=\"alice\""); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if v.Path != "name" {
		t.Errorf("Path = %q, want %q", v.Path, "name")
	}
	if v.Value != "alice" {
		t.Errorf("Value = %v, want \"alice\"", v.Value)
	}
}

func TestValue_Set_NumberValue(t *testing.T) {
	var v Value
	if err := v.Set("age=30"); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if v.Path != "age" {
		t.Errorf("Path = %q, want %q", v.Path, "age")
	}
	if v.Value != float64(30) {
		t.Errorf("Value = %v (%T), want float64(30)", v.Value, v.Value)
	}
}

func TestValue_Set_NestedPath(t *testing.T) {
	var v Value
	if err := v.Set("contact.name=\"alice\""); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if v.Path != "contact.name" {
		t.Errorf("Path = %q, want %q", v.Path, "contact.name")
	}
	if v.Value != "alice" {
		t.Errorf("Value = %v, want \"alice\"", v.Value)
	}
}

func TestValue_Set_BoolValue(t *testing.T) {
	var v Value
	if err := v.Set("enabled=true"); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if v.Value != true {
		t.Errorf("Value = %v (%T), want true", v.Value, v.Value)
	}
}

func TestValue_Set_InvalidFormat(t *testing.T) {
	var v Value
	if err := v.Set("noequalshere"); err == nil {
		t.Error("expected error for missing '='")
	}
}

func TestValue_Set_InvalidJSON(t *testing.T) {
	var v Value
	if err := v.Set("key={not json"); err == nil {
		t.Error("expected error for invalid JSON value")
	}
}

func TestValue_Type(t *testing.T) {
	var v Value
	if got := v.Type(); got != "value" {
		t.Errorf("Type() = %q, want %q", got, "value")
	}
}

func TestOptions_Data_AppliesValues(t *testing.T) {
	dir := t.TempDir()
	yamlPath := writeFile(t, dir, "a.yaml", "name: alice\ncontact:\n  city: Boston\n")
	opts := &Options{
		DataFiles: []DataFile{{DataFormat: FormatYAML, Filename: yamlPath}},
		Values: []Value{
			{Path: "name", Value: "bob"},
			{Path: "contact.city", Value: "Seattle"},
			{Path: "extra", Value: float64(42)},
		},
		Interface: log.Log,
	}
	data, err := opts.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if data["name"] != "bob" {
		t.Errorf("name = %v, want \"bob\" (--set should override file value)", data["name"])
	}
	contact, ok := data["contact"].(map[string]any)
	if !ok {
		t.Fatalf("contact = %v (%T), want map", data["contact"], data["contact"])
	}
	if contact["city"] != "Seattle" {
		t.Errorf("contact.city = %v, want \"Seattle\"", contact["city"])
	}
	if data["extra"] != float64(42) {
		t.Errorf("extra = %v, want 42", data["extra"])
	}
}

func TestEndToEnd_OverrideOrderRendersThroughTemplate(t *testing.T) {
	// Higher-level check: data from multiple files reaches the template
	// in the expected merged form.
	dir := t.TempDir()
	yamlPath := writeFile(t, dir, "a.yaml", "format: yaml\ncontact:\n  name: John\n")
	tomlPath := writeFile(t, dir, "b.toml", "format = \"toml\"\n")
	jsonPath := writeFile(t, dir, "c.json", `{"format":"json"}`)
	tmplPath := writeFile(t, dir, "t.txt", "fmt={{.format}} name={{.contact.name}}")

	opts := &Options{
		DataFiles: []DataFile{
			{DataFormat: FormatYAML, Filename: yamlPath},
			{DataFormat: FormatTOML, Filename: tomlPath},
			{DataFormat: FormatJSON, Filename: jsonPath},
		},
		TemplateFile: tmplPath,
		Interface:    log.Log,
	}

	data, err := opts.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	tmpl, err := opts.Template()
	if err != nil {
		t.Fatalf("Template() error: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if got, want := buf.String(), "fmt=json name=John"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
