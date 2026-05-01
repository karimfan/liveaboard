// Package config is the only place in the application allowed to call
// os.Getenv or read configuration files. Everything else takes values from
// the typed Config struct.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// envSource captures where a final value came from. We use this to enforce
// production rules (e.g., secrets must come from the process env, not a file).
type envSource int

const (
	srcDefault envSource = iota
	srcModeFile
	srcLocalFile
	srcProcess
)

func (s envSource) String() string {
	switch s {
	case srcModeFile:
		return "mode file"
	case srcLocalFile:
		return ".env.local"
	case srcProcess:
		return "process env"
	default:
		return "default"
	}
}

// resolved holds the final value of a single env key plus where it came from.
type resolved struct {
	value  string
	source envSource
}

type envMap map[string]resolved

// parseEnvFile reads a tiny .env-style format:
//   - blank lines and lines starting with '#' are ignored
//   - lines may start with `export `
//   - each remaining line is KEY=VALUE
//   - values are taken verbatim after the first '='; surrounding whitespace
//     is trimmed; a single pair of leading/trailing matching quotes is removed
//   - no shell expansion, no command substitution
//
// Errors include the file path and line number for any malformed line.
func parseEnvFile(r io.Reader, path string) (map[string]string, error) {
	out := map[string]string{}
	scan := bufio.NewScanner(r)
	lineNo := 0
	for scan.Scan() {
		lineNo++
		raw := strings.TrimSpace(scan.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		raw = strings.TrimPrefix(raw, "export ")
		eq := strings.IndexByte(raw, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("config: %s:%d: malformed line", path, lineNo)
		}
		key := strings.TrimSpace(raw[:eq])
		val := strings.TrimSpace(raw[eq+1:])
		if l := len(val); l >= 2 {
			first, last := val[0], val[l-1]
			if (first == '"' || first == '\'') && first == last {
				val = val[1 : l-1]
			}
		}
		out[key] = val
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	return out, nil
}

func loadFileIfPresent(path string, src envSource, into envMap) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	pairs, err := parseEnvFile(f, path)
	if err != nil {
		return err
	}
	for k, v := range pairs {
		into[k] = resolved{value: v, source: src}
	}
	return nil
}

// applyProcessEnv overlays values from the process environment for any key
// that already appears in the map (i.e., a key declared by the schema). We
// only consider keys that some struct field claims; this prevents arbitrary
// process-env leakage into Config.
func applyProcessEnv(declaredKeys []string, getenv func(string) string, into envMap) {
	for _, k := range declaredKeys {
		if v := getenv(k); v != "" {
			into[k] = resolved{value: v, source: srcProcess}
		}
	}
}

// bindStruct walks the destination struct's fields and fills them from the
// resolved env map. Field tags:
//
//	env:"KEY"          // env var name
//	default:"value"    // default if not present anywhere
//	required:"true"    // must end up non-empty (after defaulting)
//	secret:"true"      // value redacted in String(); production rules apply
//
// Returns:
//   - the populated value (via dst pointer)
//   - the set of declared env keys (for applyProcessEnv to scope which keys
//     it considers)
//   - the set of fields that ended up sourced from a non-process layer
//     (used for production-mode enforcement of secrets)
type fieldMeta struct {
	envKey    string
	required  bool
	secret    bool
	hasDflt   bool
	dflt      string
	source    envSource
	finalText string
}

// declaredKeys returns the env-tag-declared keys on dst's exported fields.
// Used to scope which process-env keys we consider before binding.
func declaredKeys(dst any) ([]string, error) {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("config: declaredKeys: want *struct, got %T", dst)
	}
	t := v.Elem().Type()
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if k := t.Field(i).Tag.Get("env"); k != "" {
			out = append(out, k)
		}
	}
	return out, nil
}

func bindStruct(dst any, env envMap) ([]string, []fieldMeta, error) {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("config: bindStruct: want *struct, got %T", dst)
	}
	v = v.Elem()
	t := v.Type()

	declared := make([]string, 0, t.NumField())
	metas := make([]fieldMeta, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		key := sf.Tag.Get("env")
		if key == "" {
			continue
		}
		dflt, hasDflt := sf.Tag.Lookup("default")
		required := sf.Tag.Get("required") == "true"
		secret := sf.Tag.Get("secret") == "true"

		declared = append(declared, key)

		text := ""
		src := srcDefault
		if r, ok := env[key]; ok {
			text = r.value
			src = r.source
		} else if hasDflt {
			text = dflt
		}

		if required && text == "" {
			return declared, metas, fmt.Errorf("config: %s is required (set in process env or %s)", key, "config/<mode>.env")
		}

		if err := setFieldFromString(v.Field(i), text); err != nil {
			return declared, metas, fmt.Errorf("config: %s: %w", key, err)
		}

		metas = append(metas, fieldMeta{
			envKey:    key,
			required:  required,
			secret:    secret,
			hasDflt:   hasDflt,
			dflt:      dflt,
			source:    src,
			finalText: text,
		})
	}

	return declared, metas, nil
}

func setFieldFromString(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		if s == "" {
			v.SetBool(false)
			return nil
		}
		b, err := strconv.ParseBool(s)
		if err != nil {
			return fmt.Errorf("invalid bool %q", s)
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// time.Duration is int64 underneath; detect by type.
		if v.Type() == reflect.TypeOf(time.Duration(0)) {
			if s == "" {
				v.SetInt(0)
				return nil
			}
			d, err := time.ParseDuration(s)
			if err != nil {
				return fmt.Errorf("invalid duration %q", s)
			}
			v.SetInt(int64(d))
			return nil
		}
		if s == "" {
			v.SetInt(0)
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int %q", s)
		}
		v.SetInt(n)
	default:
		return fmt.Errorf("unsupported field kind %s", v.Kind())
	}
	return nil
}
