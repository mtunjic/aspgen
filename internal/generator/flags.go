package generator

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// matchOption scans args for key in any of its three accepted forms
// (--key value, --key:value, -key:value) and returns the raw value found.
// ok is false when the flag is absent; err is set when the space-separated
// form is used but no value follows.
func matchOption(args []string, key string) (val string, ok bool, err error) {
	short := "-" + strings.TrimPrefix(key, "--")
	for i, arg := range args {
		if arg == key || arg == short {
			if i+1 >= len(args) {
				return "", true, fmt.Errorf("%s value is required", strings.TrimPrefix(key, "--"))
			}
			return args[i+1], true, nil
		}
		if v, found := strings.CutPrefix(arg, key+":"); found {
			return v, true, nil
		}
		if v, found := strings.CutPrefix(arg, short+":"); found {
			return v, true, nil
		}
	}
	return "", false, nil
}

func themeOption(args []string) (string, error) {
	v, ok, err := matchOption(args, "--theme")
	if err != nil || !ok {
		return "", err
	}
	return validateTheme(v)
}

func backendOption(args []string) (string, error) {
	v, ok, err := matchOption(args, "--backend")
	if err != nil || !ok {
		return "", err
	}
	return validateBackend(v)
}

func seedOption(args []string) (string, int, error) {
	for i, arg := range args {
		for _, key := range []string{"--seed", "-seed"} {
			if arg == key {
				if i+1 >= len(args) {
					return "", 0, errors.New("seed value is required")
				}
				count := 3
				if i+2 < len(args) && !strings.HasPrefix(args[i+2], "-") {
					parsed, err := strconv.Atoi(args[i+2])
					if err != nil {
						return "", 0, fmt.Errorf("invalid seed count %q", args[i+2])
					}
					count = parsed
				}
				seed, err := validateSeed(args[i+1])
				return seed, count, validateSeedCount(seed, count, err)
			}
			if strings.HasPrefix(arg, key+":") {
				value := strings.TrimPrefix(arg, key+":")
				parts := strings.Split(value, ":")
				count := 3
				if len(parts) > 1 {
					parsed, err := strconv.Atoi(parts[1])
					if err != nil {
						return "", 0, fmt.Errorf("invalid seed count %q", parts[1])
					}
					count = parsed
				}
				seed, err := validateSeed(parts[0])
				return seed, count, validateSeedCount(seed, count, err)
			}
		}
	}
	return "", 0, nil
}

func validateSeedCount(seed string, count int, seedErr error) error {
	if seedErr != nil {
		return seedErr
	}
	if seed == "" || seed == "none" {
		return nil
	}
	if count < 1 || count > 10000 {
		return fmt.Errorf("seed count must be between 1 and 10000, got %d", count)
	}
	return nil
}

func validateSeed(seed string) (string, error) {
	if seed == "" || seed == "none" {
		return "", nil
	}
	if seed != "dummy" {
		return "", fmt.Errorf("unsupported seed profile %q; use dummy", seed)
	}
	return seed, nil
}

func databaseOption(args []string) (string, error) {
	v, ok, err := matchOption(args, "--database")
	if err != nil || !ok {
		return "", err
	}
	return validateDatabase(v)
}

func providerOption(args []string) (string, error) {
	v, ok, err := matchOption(args, "--provider")
	if err != nil || !ok {
		return "", nil
	}
	for _, p := range []string{"sqlite", "postgres", "sqlserver", "mysql"} {
		if v == p {
			return v, nil
		}
	}
	return "", fmt.Errorf("unsupported provider %q; use sqlite, postgres, sqlserver, or mysql", v)
}

// connectionOption and scriptOption are mutually exclusive; validateDBSource
// enforces exactly one is present whenever DB-driven entity import is
// requested.
func connectionOption(args []string) (string, error) {
	v, _, err := matchOption(args, "--connection")
	return v, err
}

func scriptOption(args []string) (string, error) {
	v, _, err := matchOption(args, "--script")
	return v, err
}

// tablesOption returns nil (meaning "all tables") when --tables is absent
// or set to "all"; otherwise it returns the requested table names.
func tablesOption(args []string) ([]string, error) {
	v, ok, err := matchOption(args, "--tables")
	if err != nil || !ok || v == "" || v == "all" {
		return nil, err
	}
	return strings.Split(v, ","), nil
}

func validateDatabase(database string) (string, error) {
	if database == "" || database == "none" {
		return "", nil
	}
	if database != "sqlite" && database != "postgres" {
		return "", fmt.Errorf("unsupported database %q; use sqlite or postgres", database)
	}
	return database, nil
}

func validateBackend(backend string) (string, error) {
	if backend == "" || backend == "none" {
		return "", nil
	}
	if backend != "ddd" {
		return "", fmt.Errorf("unsupported backend %q; use ddd", backend)
	}
	return backend, nil
}

func validateTheme(theme string) (string, error) {
	if theme == "" || theme == "none" {
		return "", nil
	}
	if theme != "wpfui" {
		return "", fmt.Errorf("unsupported WPF theme %q; use wpfui", theme)
	}
	return theme, nil
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

// value returns the string value for key, accepting the same three forms as
// matchOption (--key value, --key:value, -key:value). Returns fallback when
// the flag is absent or given without a value in the space-separated form.
func value(args []string, key, fallback string) string {
	v, ok, err := matchOption(args, key)
	if err != nil || !ok {
		return fallback
	}
	return v
}
func templateDir(args []string) string { return value(args, "--templates", "") }

// hasFlag reports whether a boolean flag is present as --flag or -flag.
func hasFlag(args []string, flag string) bool {
	short := "-" + strings.TrimPrefix(flag, "--")
	for _, arg := range args {
		if arg == flag || arg == short {
			return true
		}
	}
	return false
}

// isHelp reports whether s is a request for help (-h, --help, or help).
func isHelp(s string) bool { return s == "-h" || s == "--help" || s == "help" }

func appendUnique(xs []string, value string) []string {
	for _, x := range xs {
		if x == value {
			return xs
		}
	}
	return append(xs, value)
}
