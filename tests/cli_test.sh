#!/usr/bin/env bash
# Functional tests for the gotmplt CLI. Builds the binary, exercises each
# command-line option (including stdin paths), and prints a pass/fail summary.
# Exits 0 if every case passes, non-zero otherwise.
set -u

here="$(cd "$(dirname "$0")" >/dev/null && pwd)"
root="$(cd "$here/.." >/dev/null && pwd)"
examples="$root/examples"
bindir="$(mktemp -d)"
bin="$bindir/gotmplt"
work="$(mktemp -d)"
trap 'rm -rf "$bindir" "$work"' EXIT

pass=0
fail=0

assert_eq() {
	# assert_eq NAME EXPECTED ACTUAL
	local name="$1" expected="$2" actual="$3"
	if [ "$actual" = "$expected" ]; then
		printf 'PASS  %s\n' "$name"
		pass=$((pass+1))
	else
		printf 'FAIL  %s\n      expected: %q\n      actual:   %q\n' "$name" "$expected" "$actual"
		fail=$((fail+1))
	fi
}

run() {
	# run NAME EXPECTED -- CMD...
	local name="$1" expected="$2"
	shift 2; [ "$1" = "--" ] && shift
	assert_eq "$name" "$expected" "$("$@" 2>/dev/null)"
}

run_stdin() {
	# run_stdin NAME EXPECTED STDIN -- CMD...
	local name="$1" expected="$2" stdin="$3"
	shift 3; [ "$1" = "--" ] && shift
	assert_eq "$name" "$expected" "$(printf '%s' "$stdin" | "$@" 2>/dev/null)"
}

run_fails() {
	# run_fails NAME -- CMD...
	local name="$1"
	shift; [ "$1" = "--" ] && shift
	if "$@" >/dev/null 2>&1; then
		printf 'FAIL  %s (expected non-zero exit)\n' "$name"
		fail=$((fail+1))
	else
		printf 'PASS  %s\n' "$name"
		pass=$((pass+1))
	fi
}

echo "building gotmplt..."
( cd "$root" && go build -o "$bin" . ) || { echo "build failed"; exit 1; }

# Fixture templates
echo '{{.format}}'           >"$work/fmt.tmpl"
echo '{{.contact.name}}'     >"$work/name.tmpl"
echo '{{.database.server}}'  >"$work/server.tmpl"
echo '{{.version}}'          >"$work/version.tmpl"
echo '{{.greeting | upper}}' >"$work/upper.tmpl"
echo '{{.a.b.c}}'            >"$work/nested.tmpl"
echo '{{.flag}}'             >"$work/flag.tmpl"
echo '{}'                    >"$work/empty.json"

# --- single data file per format ---
run "yaml file"  yaml -- "$bin" -d "$examples/data.yaml" "$work/fmt.tmpl"
run "toml file"  toml -- "$bin" -d "$examples/data.toml" "$work/fmt.tmpl"
run "json file"  json -- "$bin" -d "$examples/data.json" "$work/fmt.tmpl"

# --- field access across formats ---
run "yaml nested field" "John Doe"       -- "$bin" -d "$examples/data.yaml" "$work/name.tmpl"
run "toml nested field" "localhost:5432" -- "$bin" -d "$examples/data.toml" "$work/server.tmpl"
run "json scalar field" "1"              -- "$bin" -d "$examples/data.json" "$work/version.tmpl"

# --- merge order: last data file wins for overlapping keys ---
run "merge: last wins" json -- \
	"$bin" -d "$examples/data.yaml" -d "$examples/data.toml" -d "$examples/data.json" "$work/fmt.tmpl"
run "merge: preserves earlier keys" "John Doe" -- \
	"$bin" -d "$examples/data.yaml" -d "$examples/data.toml" -d "$examples/data.json" "$work/name.tmpl"

# --- explicit format suffix overrides extension inference ---
cp "$examples/data.json" "$work/noext"
run "filename,format suffix" json -- \
	"$bin" -d "$work/noext,json" "$work/fmt.tmpl"

# --- --set with various JSON value types ---
run "set string value"   bob        -- "$bin" -d "$work/empty.json" -s 'format="bob"'      "$work/fmt.tmpl"
run "set number value"   42         -- "$bin" -d "$work/empty.json" -s 'version=42'        "$work/version.tmpl"
run "set bool value"     true       -- "$bin" -d "$work/empty.json" -s 'flag=true'         "$work/flag.tmpl"
run "set nested path"    deep       -- "$bin" -d "$work/empty.json" -s 'a.b.c="deep"'      "$work/nested.tmpl"
run "set overrides file" overridden -- "$bin" -d "$examples/data.yaml" -s 'format="overridden"' "$work/fmt.tmpl"
run "multiple --set + sprig pipe" "HI ALICE" -- \
	"$bin" -d "$work/empty.json" -s 'greeting="hi alice"' "$work/upper.tmpl"

# --- stdin as data file (-d -,format) ---
run_stdin "stdin yaml data" "yaml" "format: yaml"       -- "$bin" -d '-,yaml' "$work/fmt.tmpl"
run_stdin "stdin json data" "json" '{"format":"json"}'  -- "$bin" -d '-,json' "$work/fmt.tmpl"
run_stdin "stdin toml data" "toml" 'format = "toml"'    -- "$bin" -d '-,toml' "$work/fmt.tmpl"

# --- template from stdin (no positional template arg) ---
run_stdin "template from stdin" "yaml" '{{.format}}' -- \
	"$bin" -d "$examples/data.yaml"

# --- both data and template from stdin is an error ---
run_fails "stdin data + stdin template rejected" -- \
	bash -c "echo '{}' | '$bin' -d '-,json'"

# --- --outfile writes output to a file ---
out="$work/out.txt"
"$bin" -d "$examples/data.yaml" -o "$out" "$work/fmt.tmpl" 2>/dev/null
assert_eq "--outfile writes to file" "yaml" "$(cat "$out")"

# --- short flag aliases ---
run "short -d" yaml -- "$bin" -d "$examples/data.yaml" "$work/fmt.tmpl"
run "short -s" bob  -- "$bin" -d "$work/empty.json" -s 'format="bob"' "$work/fmt.tmpl"
short_out="$work/short-o.txt"
"$bin" -d "$examples/data.yaml" -o "$short_out" "$work/fmt.tmpl" 2>/dev/null
assert_eq "short -o" "yaml" "$(cat "$short_out")"

# --- --functions lists Sprig functions and exits ---
funcs_out="$("$bin" --functions 2>/dev/null)"
if echo "$funcs_out" | grep -q '^Available template functions:' \
	&& echo "$funcs_out" | grep -q -- '- upper$' \
	&& echo "$funcs_out" | grep -q -- '- toPrettyJson$'; then
	printf 'PASS  --functions lists sprig functions\n'; pass=$((pass+1))
else
	printf 'FAIL  --functions output unexpected\n%s\n' "$funcs_out"; fail=$((fail+1))
fi
run "short -f also lists functions" "yes" -- \
	bash -c "'$bin' -f 2>/dev/null | grep -q '^Available template functions:' && echo yes"

# --- --debug: stdout stays clean, stderr gets debug lines ---
debug_stdout="$("$bin" --debug -d "$examples/data.yaml" "$work/fmt.tmpl" 2>/dev/null)"
assert_eq "--debug stdout clean" "yaml" "$debug_stdout"
debug_stderr="$("$bin" --debug -d "$examples/data.yaml" "$work/fmt.tmpl" 2>&1 >/dev/null)"
if echo "$debug_stderr" | grep -qi 'debug'; then
	printf 'PASS  --debug emits debug logs on stderr\n'; pass=$((pass+1))
else
	printf 'FAIL  --debug stderr missing debug lines: %q\n' "$debug_stderr"; fail=$((fail+1))
fi
quiet_stderr="$("$bin" -d "$examples/data.yaml" "$work/fmt.tmpl" 2>&1 >/dev/null)"
assert_eq "no --debug: stderr quiet" "" "$quiet_stderr"

# --- error paths ---
run_fails "missing data file"            -- "$bin" -d "$work/does-not-exist.yaml" "$work/fmt.tmpl"
run_fails "bogus explicit format"        -- "$bin" -d "$examples/data.yaml,xml" "$work/fmt.tmpl"
run_fails "unrecognized extension"       -- "$bin" -d "$work/noext" "$work/fmt.tmpl"
run_fails "missing template file"        -- "$bin" -d "$examples/data.yaml" "$work/no-such-template.tmpl"
run_fails "invalid --set (no =)"         -- "$bin" -d "$examples/data.yaml" -s 'just-a-key' "$work/fmt.tmpl"
run_fails "invalid --set (bad JSON RHS)" -- "$bin" -d "$examples/data.yaml" -s 'key={not json' "$work/fmt.tmpl"

# --- end-to-end render against the bundled template ---
e2e="$("$bin" -d "$examples/data.yaml" -d "$examples/data.json" -d "$examples/data.toml" "$examples/template.txt" 2>/dev/null)"
if echo "$e2e" | grep -q 'toml format' \
	&& echo "$e2e" | grep -q 'John Doe' \
	&& echo "$e2e" | grep -q 'localhost:5432' \
	&& echo "$e2e" | grep -q 'Version: 1'; then
	printf 'PASS  kitchen-sink render\n'; pass=$((pass+1))
else
	printf 'FAIL  kitchen-sink render unexpected output:\n%s\n' "$e2e"; fail=$((fail+1))
fi

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
