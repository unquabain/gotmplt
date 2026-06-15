package main

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/pflag"
	yaml "gopkg.in/yaml.v3"
)

type Data = map[string]any

type DataFormat int

const (
	FormatYAML DataFormat = iota
	FormatTOML
	FormatJSON
)

func (df DataFormat) Parse(instream io.Reader, data *Data) (*Data, error) {
	switch df {
	case FormatYAML:
		decoder := yaml.NewDecoder(instream)
		if err := decoder.Decode(data); err != nil {
			return nil, err
		}
	case FormatTOML:
		decoder := toml.NewDecoder(instream)
		if err := decoder.Decode(data); err != nil {
			return nil, err
		}
	case FormatJSON:
		decoder := json.NewDecoder(instream)
		if err := decoder.Decode(data); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported data format")
	}
	return data, nil
}

func (df DataFormat) String() string {
	switch df {
	case FormatYAML:
		return "YAML"
	case FormatTOML:
		return "TOML"
	case FormatJSON:
		return "JSON"
	default:
		return "Unknown"
	}
}

type DataFile struct {
	DataFormat DataFormat
	Filename   string
}

func (df *DataFile) String() string {
	return fmt.Sprintf("%s (%s)", df.Filename, df.DataFormat)
}

func (df *DataFile) Set(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) > 1 {
		filename := parts[0]
		format := parts[1]
		switch format {
		case "yaml":
			df.DataFormat = FormatYAML
		case "toml":
			df.DataFormat = FormatTOML
		case "json":
			df.DataFormat = FormatJSON
		default:
			return fmt.Errorf("unsupported file format: %s", format)
		}
		df.Filename = filename
		return nil
	}
	switch {
	case len(value) > 5 && value[len(value)-5:] == ".yaml":
		df.DataFormat = FormatYAML
	case len(value) > 4 && value[len(value)-4:] == ".yml":
		df.DataFormat = FormatYAML
	case len(value) > 5 && value[len(value)-5:] == ".toml":
		df.DataFormat = FormatTOML
	case len(value) > 5 && value[len(value)-5:] == ".json":
		df.DataFormat = FormatJSON
	default:
		return fmt.Errorf("unsupported file format for %s", value)
	}
	df.Filename = value
	return nil
}

func (df *DataFile) Type() string {
	return "datafile"
}

type Value struct {
	Path  string
	Value any
}

func (v *Value) String() string {
	return fmt.Sprintf("%s: %v", v.Path, v.Value)
}

func (v *Value) Type() string {
	return "value"
}

func (v *Value) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid value specification: %s", value)
	}
	v.Path = parts[0]
	var parsed any
	if err := json.Unmarshal([]byte(parts[1]), &parsed); err != nil {
		return fmt.Errorf("failed to parse value: %w", err)
	}
	v.Value = parsed
	return nil
}

type Options struct {
	DataFiles    []DataFile
	Values       []Value
	TemplateFile string
	Debug        bool
	Outfile      *string
	log.Interface
}

func ParseOptions() (*Options, error) {
	var dataFileSpecs []string
	var dataFiles []DataFile
	var valueSpecs []string
	var values []Value
	var templateFile string
	var debug bool
	var functions bool
	var stdinUsed bool
	var outfile = new(string)

	log.SetHandler(text.New(os.Stderr))

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gotmplt [flags] [template-file]\n\n")
		fmt.Fprintf(os.Stderr, "Renders a Go template with data from YAML, TOML, or JSON files.\n\n")
		fmt.Fprintf(os.Stderr, "If template-file is omitted, the template is read from stdin.\n")
		fmt.Fprintf(os.Stderr, "Stdin cannot be used for both a data file (-d -,format) and the template.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		pflag.PrintDefaults()
	}

	pflag.StringArrayVarP(&dataFileSpecs, "data", "d", []string{}, "data file (YAML, TOML, JSON); use '-,format' to read from stdin; use 'filename,format' to force format")
	pflag.StringArrayVarP(&valueSpecs, "set", "s", []string{}, "set a single value (key=value) where value is JSON; can be used multiple times")
	pflag.BoolVarP(&debug, "debug", "g", false, "enable debug logging")
	pflag.BoolVarP(&functions, "functions", "f", false, "list available Sprig template functions and exit")
	pflag.StringVarP(outfile, "outfile", "o", "", "write output to file instead of stdout")

	pflag.Parse()

	if functions {
		fmt.Println("Available template functions:")
		fmt.Println("   see https://masterminds.github.io/sprig/ for details")
		for _, name := range slices.Sorted(maps.Keys(sprig.TxtFuncMap())) {
			fmt.Println(" -", name)
		}
		os.Exit(0)
	}

	dataFiles = make([]DataFile, len(dataFileSpecs))
	for i, spec := range dataFileSpecs {
		var df DataFile
		if err := df.Set(spec); err != nil {
			return nil, fmt.Errorf("invalid data file specification: %w", err)
		}
		stdinUsed = stdinUsed || df.Filename == "-"
		dataFiles[i] = df
	}

	values = make([]Value, len(valueSpecs))
	for i, spec := range valueSpecs {
		var v Value
		if err := v.Set(spec); err != nil {
			return nil, fmt.Errorf("invalid value specification: %w", err)
		}
		values[i] = v
	}

	if pflag.NArg() < 1 {
		if stdinUsed {
			return nil, fmt.Errorf("template file is required")
		}
		templateFile = "-"
	} else {
		templateFile = pflag.Arg(0)
	}
	if templateFile == "-" && stdinUsed {
		return nil, fmt.Errorf("cannot read both data and template from stdin")
	}

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	return &Options{
		DataFiles:    dataFiles,
		Values:       values,
		TemplateFile: templateFile,
		Debug:        debug,
		Outfile:      outfile,
		Interface:    log.Log,
	}, nil
}

func (opts *Options) Data() (Data, error) {
	opts.Debugf("loading data from %d file(s)", len(opts.DataFiles))
	data := make(Data)
	var file io.ReadCloser
	var err error
	for _, df := range opts.DataFiles {
		opts.Debugf("opening data file %s", df.String())
		if df.Filename == "-" {
			opts.Debugf("reading data from stdin")
			file = os.Stdin
		} else {
			file, err = os.Open(df.Filename)
			if err != nil {
				return nil, fmt.Errorf("failed to open data file %s: %w", df.Filename, err)
			}
		}
		defer file.Close()

		opts.Debugf("parsing data file %s as %s", df.Filename, df.DataFormat)
		if _, err := df.DataFormat.Parse(file, &data); err != nil {
			return nil, fmt.Errorf("failed to parse data file %s: %w", df.Filename, err)
		}

	}
	for _, v := range opts.Values {
		opts.Debugf("setting value %s", v.String())
		if err := digSet(data, v.Path, v.Value); err != nil {
			return nil, fmt.Errorf("failed to set value %s: %w", v.String(), err)
		}
	}
	opts.Debugf("loaded %d top-level data keys", len(data))
	return data, nil
}

func (opts *Options) OutStream() (io.WriteCloser, error) {
	if opts.Outfile != nil && *opts.Outfile != "" {
		opts.Debugf("creating output file %s", *opts.Outfile)
		file, err := os.Create(*opts.Outfile)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file %s: %w", *opts.Outfile, err)
		}
		return file, nil
	}
	opts.Debugf("no outfile specified; using stdout")
	return os.Stdout, nil
}

func (opts *Options) Template() (*template.Template, error) {
	var content []byte
	var err error
	if opts.TemplateFile == "" {
		opts.Debugf("no template file provided")
		return nil, fmt.Errorf("template file is required")
	}
	opts.Debugf("reading template file %s", opts.TemplateFile)
	if opts.TemplateFile == "-" {
		opts.Debugf("reading template from stdin")
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read template from stdin: %w", err)
		}
	} else {
		content, err = os.ReadFile(opts.TemplateFile)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read template file %s: %w", opts.TemplateFile, err)
	}
	opts.Debugf("parsing template %s (%d bytes)", opts.TemplateFile, len(content))
	tmpl, err := template.New(opts.TemplateFile).Funcs(sprig.TxtFuncMap()).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template file %s: %w", opts.TemplateFile, err)
	}
	return tmpl, nil
}

type Must[T any] struct {
	Value T
	Err   error
}

func ifErr(err error) *Must[struct{}] {
	return &Must[struct{}]{Err: err}
}

func must[T any](value T, err error) *Must[T] {
	return &Must[T]{Value: value, Err: err}
}

func (m *Must[T]) OrDie(msg string, args ...any) T {
	if m.Err != nil {
		log.WithError(m.Err).Fatalf(msg, args...)
	}
	return m.Value
}

func (m *Must[T]) OrDefault(defaultValue T) T {
	if m.Err != nil {
		return defaultValue
	}
	return m.Value
}

func main() {
	opts := must(ParseOptions()).OrDie("Failed to parse options")
	data := must(opts.Data()).OrDie("Failed to load data")
	tmpl := must(opts.Template()).OrDie("Failed to load template")
	outStream := must(opts.OutStream()).OrDie("Failed to open output stream")
	defer outStream.Close()

	ifErr(tmpl.Execute(outStream, data)).OrDie("Failed to execute template")
}
