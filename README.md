# gotmplt

A CLI tool for rendering Go templates with data from YAML, TOML, and JSON files. Supports the full [Sprig](https://masterminds.github.io/sprig/) function library.

## Installation

```sh
go install github.com/unquabain/gotmplt@latest
```

## Usage

```
gotmplt [flags] [template-file]
```

If `template-file` is omitted, the template is read from **stdin**. Stdin cannot be used for both a data file and the template simultaneously.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--data <file>` | `-d` | Data file to load (repeatable; later files override earlier keys) |
| `--set <path=json>` | `-s` | Set a single value at a dotted path; value is parsed as JSON (repeatable) |
| `--outfile <file>` | `-o` | Write output to a file instead of stdout |
| `--functions` | `-f` | List all available Sprig template functions and exit |
| `--debug` | `-g` | Enable debug logging |

### Data files

Supported formats: **YAML** (`.yaml`, `.yml`), **TOML** (`.toml`), **JSON** (`.json`).

Format is inferred from the file extension. To specify it explicitly, use `filename,format`:

```sh
gotmplt -d mydata,yaml template.txt
```

To read data from **stdin**, use `-` as the filename with an explicit format:

```sh
echo '{"name": "world"}' | gotmplt -d -,json template.txt
```

To read the **template** from stdin, omit the template file argument:

```sh
echo 'Hello, {{.name}}!' | gotmplt -d data.yaml
```

Multiple `-d` flags are merged in order — later files overwrite conflicting keys.

### Setting values from the command line

Use `--set` (`-s`) to override or inject individual values without editing a data file. The argument is `path=json`, where `path` is a dot-separated key path and `json` is a JSON literal:

```sh
gotmplt -d data.yaml \
  -s 'format="overridden"' \
  -s 'version=42' \
  -s 'feature.enabled=true' \
  -s 'contact.address.city="Boston"' \
  template.txt
```

`--set` runs after all `-d` files load, so it overrides them. Intermediate maps along the path are created as needed.

## Example

Given these data files and a template:

**data.yaml**
```yaml
format: yaml
contact:
  name: "John Doe"
  email: "jdoe@example.com"
```

**data.toml**
```toml
format = "toml"
[database]
server = "localhost:5432"
```

**data.json**
```json
{ "format": "json", "version": 1 }
```

**template.txt**
```
The most recent data file was in {{.format}} format.

Contact data:
	name: {{.contact.name}}
	email: {{.contact.email}}

Contact JSON:
{{.contact | toPrettyJson}}

Version: {{.version}}

Database:
	server: {{.database.server | quote }}
```

Run:

```sh
gotmplt -d data.yaml -d data.toml -d data.json template.txt
```

Output:

```
The most recent data file was in json format.

Contact data:
	name: John Doe
	email: jdoe@example.com

Contact JSON:
{
  "email": "jdoe@example.com",
  "name": "John Doe"
}

Version: 1

Database:
	server: "localhost:5432"
```

## Template functions

`gotmplt` exposes the full Sprig function set inside templates, plus a `jq` function for applying [jq](https://stedolan.github.io/jq/) queries to arbitrary data. To see all available functions:

```sh
gotmplt --functions
```

Full Sprig documentation: <https://masterminds.github.io/sprig/>

### `jq`

```
jq QUERY DATA
```

Applies a [gojq](https://github.com/itchyny/gojq) query string to `DATA` and returns the result. If the query produces a single value, that value is returned directly; if it produces multiple values, they are returned as a slice. This is useful for reshaping data inside a template — for example, turning a list of `{label, value}` objects into a map keyed by `label`:

```gotemplate
{{ with .fields | jq `[.[] | {"key": .label, "value": .value}] | from_entries` }}
name  = {{ .name | quote }}
email = {{ .email | quote }}
{{ end }}
```

Given input data like:

```json
{ "fields": [
  { "label": "name",  "value": "Alice" },
  { "label": "email", "value": "alice@example.com" }
] }
```

the template above renders:

```
name  = "Alice"
email = "alice@example.com"
```

## Testing

Unit tests:

```sh
go test ./...
```

CLI functional tests (builds the binary, exercises every flag including stdin paths):

```sh
bash tests/cli_test.sh
```

## Dependencies

- [Sprig](https://github.com/Masterminds/sprig) — extended template functions
- [gojq](https://github.com/itchyny/gojq) — jq query engine for the `jq` template function
- [go-toml](https://github.com/pelletier/go-toml) — TOML parsing
- [yaml.v3](https://gopkg.in/yaml.v3) — YAML parsing
- [pflag](https://github.com/spf13/pflag) — POSIX-style flags
- [apex/log](https://github.com/apex/log) — structured logging
