// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.

// all-keywords generates sql/lex/keywords.go from sql.y.
//
// It is generically structured with Go templates to allow for quick
// prototyping of different code generation structures for keyword token
// lookup. Previous attempts:
//
// Using github.com/cespare/mph to generate a perfect hash function. Was 10%
// slower. Also attempted to populate the mph.Table with a sparse array where
// the index correlated to the token id. This generated such a large array
// (~65k entries) that the mph package never returned from its Build call.
//
// A `KeywordsTokens = map[string]int32` map from string -> token id.
package main

import (
	"bufio"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

func main() {
	keywordRE := regexp.MustCompile(`^.*_keyword:`)
	pipeRE := regexp.MustCompile(`[A-Z].*`)

	keyword := false
	category := ""
	seen := map[string]bool{}
	scanner := bufio.NewScanner(os.Stdin)
	type entry struct {
		Lower, Match, Category string
	}
	var data []entry
	// Look for lines that start with "XXX_keyword:" and record the category. For
	// subsequent non-empty lines, all words are keywords so add them to our
	// data list. An empty line indicates the end of the keyword section, so
	// stop recording.
	for scanner.Scan() {
		line := scanner.Text()
		if match := keywordRE.FindString(line); match != "" {
			keyword = true
			category = categories[match]
			if category == "" {
				log.Fatal("unknown keyword type:", match)
			}
		} else if line == "" {
			keyword = false
		} else if match = pipeRE.FindString(line); keyword && match != "" && !seen[match] {
			seen[match] = true
			data = append(data, entry{
				Lower:    strings.ToLower(match),
				Match:    match,
				Category: category,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("reading standard input:", err)
	}

	sort.Slice(data, func(i, j int) bool {
		return data[i].Match < data[j].Match
	})

	if err := template.Must(template.New("tmpl").Parse(tmpl)).Execute(os.Stdout, data); err != nil {
		log.Fatal(err)
	}
}

// Category codes are for pg_get_keywords, see
// src/backend/utils/adt/misc.c in pg's sources.
var categories = map[string]string{
	"col_name_keyword:":                         "C",
	"unreserved_keyword:":                       "U",
	"type_func_name_keyword:":                   "T",
	"cockroachdb_extra_type_func_name_keyword:": "T",
	"reserved_keyword:":                         "R",
	"cockroachdb_extra_reserved_keyword:":       "R",
}

const tmpl = `// Code generated by cmd/all-keywords. DO NOT EDIT.
// GENERATED FILE DO NOT EDIT

package lex

var Keywords = map[string]struct {
	Tok int
	Cat string
}{
{{range . -}}
	"{{.Lower}}": { {{.Match}}, "{{.Category}}" },
{{end -}}
}

// GetKeywordID returns the lex id of the SQL keyword k or IDENT if k is
// not a keyword.
func GetKeywordID(k string) int32 {
	// The previous implementation generated a map that did a string ->
	// id lookup. Various ideas were benchmarked and the implementation below
	// was the fastest of those, between 3% and 10% faster (at parsing, so the
	// scanning speedup is even more) than the map implementation.
	switch k {
	{{range . -}}
	case "{{.Lower}}": return {{.Match}}
	{{end -}}
	default: return IDENT
	}
}
`
