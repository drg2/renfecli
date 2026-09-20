package main

import (
	"encoding/json"
	"fmt"
	"os"

	toon "github.com/toon-format/toon-go"
)

// The machine-readable views. Every command can emit its result as JSON or as
// TOON instead of printing a table, which is what makes the CLI usable from a
// script or an agent; --json data goes to stdout and diagnostics to stderr, so
// the two never mix.

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// toonEncode renders v as TOON. It routes through JSON first so the field names
// and shapes match --json exactly (the TOON library reads its own struct tags,
// not the json ones); map keys are sorted, so output is deterministic.
func toonEncode(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var generic any
	_ = json.Unmarshal(raw, &generic) // raw is json.Marshal output → always valid JSON
	// The TOON encoder rejects a control character rather than escaping it, so
	// one in a station name would fail the command instead of printing it.
	return toon.MarshalString(safeValue(generic))
}

func emitTOON(v any) error {
	s, err := toonEncode(v)
	if err != nil || s == "" {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, s)
	return err
}

// emitStructured emits v in the selected structured format (--toon or --json)
// and reports whether it did, so callers fall through to the human-readable
// view only when neither flag is set.
func emitStructured(cf *common, v any) (emitted bool, err error) {
	switch {
	case cf.toon:
		return true, emitTOON(v)
	case cf.jsonOut:
		return true, emitJSON(v)
	}
	return false, nil
}

// emitStream writes one record for a command that reports repeatedly rather
// than once. JSON goes out compact, one object per line — the JSON Lines a
// consumer reads a line at a time; the indented form emitStructured uses for a
// single result would make it buffer until the process ends.
func emitStream(cf *common, v any) error {
	if cf.toon {
		return emitTOON(v)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, string(b))
	return err
}
