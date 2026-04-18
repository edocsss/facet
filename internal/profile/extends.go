package profile

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ExtendsKind string

const (
	ExtendsGit  ExtendsKind = "git"
	ExtendsDir  ExtendsKind = "dir"
	ExtendsFile ExtendsKind = "file"
)

type ExtendsSpec struct {
	Raw     string
	Kind    ExtendsKind
	Locator string
	Ref     string
}

func ParseExtends(raw string) (ExtendsSpec, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ExtendsSpec{}, fmt.Errorf("extends must not be empty")
	}
	if strings.HasPrefix(trimmed, "@") {
		return ExtendsSpec{}, fmt.Errorf("extends %q is not a valid locator", raw)
	}
	if trimmed == "base" {
		return ExtendsSpec{
			Raw:     raw,
			Kind:    ExtendsFile,
			Locator: "base.yaml",
		}, nil
	}

	if locator, ref, ok := splitRef(trimmed); ok {
		return ExtendsSpec{
			Raw:     raw,
			Kind:    ExtendsGit,
			Locator: locator,
			Ref:     ref,
		}, nil
	}

	if looksLikeGitLocator(trimmed) {
		return ExtendsSpec{
			Raw:     raw,
			Kind:    ExtendsGit,
			Locator: trimmed,
		}, nil
	}

	if isYAMLPath(trimmed) {
		return ExtendsSpec{
			Raw:     raw,
			Kind:    ExtendsFile,
			Locator: trimmed,
		}, nil
	}

	if looksLikeLocalLocator(trimmed) {
		return ExtendsSpec{
			Raw:     raw,
			Kind:    ExtendsDir,
			Locator: trimmed,
		}, nil
	}

	return ExtendsSpec{}, fmt.Errorf("extends %q is not a valid locator", raw)
}

func splitRef(raw string) (string, string, bool) {
	idx := strings.LastIndex(raw, "@")
	if idx <= 0 || idx == len(raw)-1 {
		return "", "", false
	}

	locator := raw[:idx]
	ref := raw[idx+1:]
	if ref == "" || isYAMLPath(locator) {
		return "", "", false
	}
	if looksLikeGitLocator(locator) {
		return locator, ref, true
	}
	return "", "", false
}

func looksLikeGitLocator(raw string) bool {
	return strings.HasPrefix(raw, "https://") ||
		strings.HasPrefix(raw, "http://") ||
		strings.HasPrefix(raw, "ssh://") ||
		strings.HasPrefix(raw, "file://") ||
		(strings.HasPrefix(raw, "git@") && strings.Contains(raw, ":"))
}

func looksLikeLocalLocator(raw string) bool {
	if strings.ContainsAny(raw, " \t\r\n") {
		return false
	}
	return filepath.IsAbs(raw) ||
		strings.HasPrefix(raw, ".") ||
		strings.HasPrefix(raw, "~") ||
		strings.Contains(raw, "/")
}

func isYAMLPath(raw string) bool {
	return strings.HasSuffix(raw, ".yaml") || strings.HasSuffix(raw, ".yml")
}
