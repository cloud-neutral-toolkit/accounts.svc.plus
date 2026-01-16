package utils

import (
	"bufio"
	"sort"
	"strings"
)

// NormalizeStatements extracts relevant DDL statements from the provided dump.
// Comments, empty lines and helper SET statements are ignored. Whitespace is
// normalised to make comparisons deterministic across environments.
func NormalizeStatements(dump string) []string {
	var statements []string
	var builder strings.Builder

	flush := func() {
		stmt := strings.TrimSpace(builder.String())
		if stmt == "" {
			builder.Reset()
			return
		}
		stmt = strings.TrimSuffix(stmt, ";")
		stmt = strings.TrimSpace(stmt)
		if isRelevantStatement(stmt) {
			normalized := collapseWhitespace(stmt)
			statements = append(statements, normalized)
		}
		builder.Reset()
	}

	scanner := bufio.NewScanner(strings.NewReader(dump))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		if shouldSkipLine(line) {
			continue
		}

		builder.WriteString(line)
		if strings.HasSuffix(line, ";") {
			flush()
		} else {
			builder.WriteString(" ")
		}
	}

	flush()

	sort.Strings(statements)
	return statements
}

// CompareStatements returns the statements that are present only on the left
// or only on the right collection.
func CompareStatements(left, right []string) (onlyLeft, onlyRight []string) {
	leftSet := make(map[string]struct{}, len(left))
	for _, stmt := range left {
		if strings.Contains(stmt, "pglogical") {
			continue
		}
		leftSet[stmt] = struct{}{}
	}

	rightSet := make(map[string]struct{}, len(right))
	for _, stmt := range right {
		if strings.Contains(stmt, "pglogical") {
			continue
		}
		rightSet[stmt] = struct{}{}
	}

	for stmt := range leftSet {
		if _, ok := rightSet[stmt]; !ok {
			onlyLeft = append(onlyLeft, stmt)
		}
	}

	for stmt := range rightSet {
		if _, ok := leftSet[stmt]; !ok {
			onlyRight = append(onlyRight, stmt)
		}
	}

	sort.Strings(onlyLeft)
	sort.Strings(onlyRight)
	return
}

func shouldSkipLine(line string) bool {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(lower, "set "):
		return true
	case strings.HasPrefix(lower, "select pg_catalog.set_config"):
		return true
	case strings.HasPrefix(lower, "reset "):
		return true
	case strings.HasPrefix(line, "\\connect "):
		return true
	case strings.HasPrefix(lower, "lock table"):
		return true
	}
	return false
}

func isRelevantStatement(stmt string) bool {
	lower := strings.ToLower(stmt)
	switch {
	case strings.HasPrefix(lower, "create table"):
		return true
	case strings.HasPrefix(lower, "alter table"):
		return true
	case strings.HasPrefix(lower, "create index"):
		return true
	case strings.HasPrefix(lower, "alter index"):
		return true
	case strings.HasPrefix(lower, "comment on table"):
		return true
	case strings.HasPrefix(lower, "comment on column"):
		return true
	case strings.HasPrefix(lower, "grant "):
		return true
	}
	return false
}

func collapseWhitespace(input string) string {
	fields := strings.Fields(input)
	return strings.Join(fields, " ")
}
