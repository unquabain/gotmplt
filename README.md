# gotmplt

A CLI tool for rendering Go templates with data from YAML, TOML, and JSON files. Supports the full [Sprig](https://masterminds.github.io/sprig/) function library.

## Installation

```sh
go install github.com/unquabain/gotmplt@latest
```

## Usage

```
gotmplt [flags] <template-file>
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--data <file>` | `-d` | Data file to load (repeatable; later files override earlier keys) |
| `--outfile <file>` | `-o` | Write output to a file instead of stdout |
| `--functions` | `-f` | List all available Sprig template functions and exit |
| `--debug` | `-g` | Enable debug logging |

### Data files

Supported formats: **YAML** (`.yaml`, `.yml`), **TOML** (`.toml`), **JSON** (`.json`).

Format is inferred from the file extension. To specify it explicitly, use `filename,format`:

```sh
gotmplt -d mydata,yaml template.txt
```

Multiple `-d` flags are merged in order — later files overwrite conflicting keys.

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

`gotmplt` exposes the full Sprig function set inside templates. To see all available functions:

```sh
gotmplt --functions
```

Full documentation: <https://masterminds.github.io/sprig/>

## Dependencies

- [Sprig](https://github.com/Masterminds/sprig) — extended template functions
- [go-toml](https://github.com/pelletier/go-toml) — TOML parsing
- [yaml.v3](https://gopkg.in/yaml.v3) — YAML parsing
- [pflag](https://github.com/spf13/pflag) — POSIX-style flags
- [apex/log](https://github.com/apex/log) — structured logging
