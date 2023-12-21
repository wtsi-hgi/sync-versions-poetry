/*
 * Copyright (c) 2023 Genome Research Ltd.
 *
 * Author: Ash Holland <ah37@sanger.ac.uk>
 *
 * This program is free software: you can redistribute it and/or modify it under
 * the terms of the GNU General Public License as published by the Free Software
 * Foundation; either version 3 of the License, or (at your option) any later
 * version.
 *
 * This program is distributed in the hope that it will be useful, but WITHOUT
 * ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS
 * FOR A PARTICULAR PURPOSE. See the GNU General Public License for more
 * details.
 *
 * You should have received a copy of the GNU General Public License along with
 * this program. If not, see <http://www.gnu.org/licenses/>.
 */

package cmd

import (
	"fmt"
	"os"
	"slices"
	"testing"
)

// When a .pre-commit-config.yaml and poetry.lock are present, the root command should succeed.
func TestExecute(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func(dir string) {
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
	}(cwd)
	if err := os.WriteFile(".pre-commit-config.yaml", []byte{}, 0666); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("poetry.lock", []byte{}, 0666); err != nil {
		t.Fatal(err)
	}
	if err := rootCmd.Execute(); err != nil {
		t.Fail()
	}
}

// When a .pre-commit-config.yaml is absent, the root command should fail.
func TestExecuteMissingConfig(t *testing.T) {
	defer func() { _ = recover() }()
	_ = rootCmd.Execute()
	t.Fatal("did not panic")
}

// When run in a directory with a .pre-commit-config.yaml,
// readPreCommitFile() should succeed.
func TestReadPreCommitFile(t *testing.T) {
	_, err := readPreCommitFile(os.DirFS("testdata"))
	if err != nil {
		t.Fatal(err)
	}
}

// When passed a valid pre-commit file, loadPreCommitConfig() should succeed.
func TestLoadPreCommitConfig(t *testing.T) {
	config, err := loadPreCommitConfig([]byte(`
repos:
- hooks:
  - id: foo
    additional_dependencies: [a, b, c]
`))
	if err != nil {
		t.Error(err)
	}
	var id = config.Repos[0].Hooks[0].Id
	if id != "foo" {
		t.Error("wrong name:", id)
	}
	var additionalDeps = config.Repos[0].Hooks[0].AdditionalDependencies
	if len(additionalDeps) != 3 {
		t.Error("wrong deps:", additionalDeps)
	}
}

// When passed a valid poetry.lock, loadPoetryLock() should succeed.
func TestLoadPoetryLock(t *testing.T) {
	lockfile, err := loadPoetryLock(os.DirFS("testdata"))
	if err != nil {
		t.Fatal(err)
	}
	if lockfile.Metadata.LockVersion != "2.0" {
		t.Error("wrong lock-version:", lockfile.Metadata.LockVersion)
	}
}

// When passed no hooks, or a hook with no additional_dependencies, checkVersions() should succeed.
func TestCheckVersionsSimple(t *testing.T) {
	tests := []struct {
		hooks []string
	}{
		{[]string{}},
		{[]string{"golangci-lint"}},
	}
	file, err := readPreCommitFile(os.DirFS("testdata"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := loadPreCommitConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.hooks), func(t *testing.T) {
			problems := checkVersions(config, poetryLock{}, test.hooks)
			if len(problems) != 0 {
				t.Error("unexpected problems", problems)
			}
		})
	}
}

// When passed a hook with problems, checkVersions() should return the problems detected.
func TestCheckVersionsFailing(t *testing.T) {
	file, err := readPreCommitFile(os.DirFS("testdata"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := loadPreCommitConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	lockfile, err := loadPoetryLock(os.DirFS("testdata"))
	if err != nil {
		t.Fatal(err)
	}
	problems := checkVersions(config, lockfile, []string{"flake8"})
	if !slices.Equal(problems, []string{"flake8-typing-imports==1.14.0: version mismatch (expected: 1.15.0)"}) {
		t.Error("incorrect problems", problems)
	}
}

// When passed a dependency, checkVersion() should return a problem when appropriate.
func TestCheckVersion(t *testing.T) {
	tests := []struct {
		depspec string
		problem string
	}{
		// Underspecified versions are not allowed
		{"virtualenv", "empty version spec not permitted"},
		{"virtualenv>=20.25,<21", "must specify an exact version (expected: virtualenv==20.25.0)"},
		{"virtualenv===20.25.0", "arbitrary equality (===) not permitted (expected: virtualenv==20.25.0)"},
		{"virtualenv==20.25.*", "trailing .* not permitted"},
		// Only an exact version matching clause is allowed...
		{"virtualenv==20.25.0", ""},
		{"virtualenv==20.25", ""},
		// ...and only as long as it matches what's in poetry.lock.
		{"virtualenv==20.24.0", "version mismatch (expected: 20.25.0)"},
		// URLs are not allowed (because they cannot be matched against poetry.lock)
		{"virtualenv @ http://example.com#sha1=da39a3ee5e6b4b0d3255bfef95601890afd80709", "URLs not permitted"},
		// Extras are OK
		{"virtualenv[foo,bar]==20.25.*", "trailing .* not permitted"},
		{"virtualenv[foo,bar]==20.25.0", ""},
		// Environment markers are not allowed (because they make no sense with pre-commit, where you know the Python version being used)
		// (also just because they're an enormous pain to parse)
		{"virtualenv==20.25.0 ; python_version < \"3.14\"", "environment markers not permitted"},
		{"virtualenv @ https://example.com#sha1=da39a3ee5e6b4b0d3255bfef95601890afd80709 ; python_version < \"3.14\"", "URLs not permitted"},
		// Packages not in poetry.lock are not allowed
		{"does-not-exist==1.2.3", "not found in poetry.lock"},
		// Invalid dependency specifiers are not allowed
		{"this is nonsense", "invalid dependency specification"},
		{"different-nonsense==1..100", "invalid version specification"},
		// Package names are normalized
		{"FLAKE8-DocStrings==1.7", ""},
		{"flake8_typing.imports==1.15", ""},
	}
	lockfile, err := loadPoetryLock(os.DirFS("testdata"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.depspec, func(t *testing.T) {
			problem := checkVersion(test.depspec, lockfile)
			if problem != test.problem {
				t.Errorf("got %q wanted %q", problem, test.problem)
			}
		})
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// testcases from https://packaging.python.org/en/latest/specifications/name-normalization/
		{"friendly-bard", "friendly-bard"},
		{"Friendly-Bard", "friendly-bard"},
		{"FRIENDLY-BARD", "friendly-bard"},
		{"friendly.bard", "friendly-bard"},
		{"friendly_bard", "friendly-bard"},
		{"friendly--bard", "friendly-bard"},
		{"FrIeNdLy-._.-bArD", "friendly-bard"},
	}
	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			got := normalizeName(test.in)
			if got != test.want {
				t.Errorf("got %q want %q", got, test.want)
			}
		})
	}
}
