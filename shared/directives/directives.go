// Package directives provides parsing and evaluation of inline argus:ignore comments.
package directives

import (
	"go/ast"
	"go/token"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var (
	// Matches:
	//   // argus:ignore-a07 <reason>
	//   // argus:ignore-a07.detail <reason>
	//   // argus:ignore ARGUS-A07 <reason>
	//   // argus:ignore A12,A16,A23 <reason>
	//   /* argus:ignore-a07 <reason> */
	//   -- argus:ignore-a07 <reason>
	directiveRegex = regexp.MustCompile(`(?i)(?://|/\*|--)\s*argus:ignore(?:[-:\s]+)([a-z0-9_.,-]+)\s+([^*\r\n]+)`)
)

// IgnoreDirective represents a single parsed suppression directive.
type IgnoreDirective struct {
	Rule   string
	Reason string
	Line   int
}

// DirectiveMap tracks ignore directives per source file.
type DirectiveMap struct {
	// file -> line -> rule -> reason
	directives map[string]map[int]map[string]string
}

// NewDirectiveMap creates an empty DirectiveMap.
func NewDirectiveMap() *DirectiveMap {
	return &DirectiveMap{
		directives: make(map[string]map[int]map[string]string),
	}
}

// AddDirective records a directive for a specific file and line.
func (dm *DirectiveMap) AddDirective(filename string, line int, rule, reason string) {
	reason = strings.TrimSpace(reason)
	if len(strings.Fields(reason)) < 2 {
		return // Reason must have at least 2 words
	}

	for _, singleRule := range strings.Split(rule, ",") {
		singleRule = strings.TrimSpace(singleRule)
		if singleRule == "" {
			continue
		}
		canonical, base := normalizeRule(singleRule)

		if _, ok := dm.directives[filename]; !ok {
			dm.directives[filename] = make(map[int]map[string]string)
		}
		if _, ok := dm.directives[filename][line]; !ok {
			dm.directives[filename][line] = make(map[string]string)
		}
		dm.directives[filename][line][canonical] = reason
		if base != canonical {
			dm.directives[filename][line][base] = reason
		}
	}
}

// IsIgnored checks if a given violation at pos is suppressed by an ignore directive.
func (dm *DirectiveMap) IsIgnored(fset *token.FileSet, pos token.Pos, ruleCode string) bool {
	if dm == nil || fset == nil {
		return false
	}
	p := fset.Position(pos)
	return dm.IsLineIgnored(p.Filename, p.Line, ruleCode)
}

// IsLineIgnored checks whether a specific file and line is suppressed.
// Checks the exact line or up to 5 lines above to cover multi-line expressions and stacked ignore comments.
func (dm *DirectiveMap) IsLineIgnored(filename string, line int, ruleCode string) bool {
	if dm == nil {
		return false
	}
	fileMap, ok := dm.directives[filename]
	if !ok {
		return false
	}

	canonical, base := normalizeRule(ruleCode)

	for _, l := range []int{line, line - 1, line - 2, line - 3, line - 4, line - 5} {
		if rules, exists := fileMap[l]; exists {
			if _, matched := rules[canonical]; matched {
				return true
			}
			if _, matched := rules[base]; matched {
				return true
			}
			if _, all := rules["ALL"]; all {
				return true
			}
		}
	}
	return false
}

// ParseSQLDirectives parses SQL comments to find ignore directives.
func ParseSQLDirectives(content string, filename string) *DirectiveMap {
	dm := NewDirectiveMap()
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lineNum := i + 1
		matches := directiveRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			dm.AddDirective(filename, lineNum, matches[1], matches[2])
		}
	}
	return dm
}

// Analyzer collects and provides the DirectiveMap for all analyzed packages.
var Analyzer = &analysis.Analyzer{
	Name:       "argus_directives",
	Doc:        "Parses inline // argus:ignore directives across Go source files",
	Run:        runDirectivesAnalyzer,
	ResultType: reflect.TypeOf((*DirectiveMap)(nil)),
}

// ParseGoDirectives parses comments from an *ast.File into a DirectiveMap.
func ParseGoDirectives(file *ast.File, fset *token.FileSet) *DirectiveMap {
	dm := NewDirectiveMap()
	pos := fset.Position(file.Package)
	filename := pos.Filename

	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			matches := directiveRegex.FindStringSubmatch(comment.Text)
			if len(matches) == 3 {
				cPos := fset.Position(comment.Pos())
				dm.AddDirective(filename, cPos.Line, matches[1], matches[2])
			}
		}
	}
	return dm
}

func runDirectivesAnalyzer(pass *analysis.Pass) (interface{}, error) {
	dm := NewDirectiveMap()

	for _, file := range pass.Files {
		fileDm := ParseGoDirectives(file, pass.Fset)
		for fn, lines := range fileDm.directives {
			for l, rules := range lines {
				for r, reason := range rules {
					dm.AddDirective(fn, l, r, reason)
				}
			}
		}
	}
	return dm, nil
}
