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
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/aquasecurity/go-pep440-version"
	"github.com/pelletier/go-toml"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var rootCmd = &cobra.Command{
	Use: "sync-versions-poetry",
	Run: func(cmd *cobra.Command, args []string) {
		file, err := readPreCommitFile(os.DirFS("."))
		if err != nil {
			panic(err)
		}
		data, err := loadPreCommitConfig(file)
		if err != nil {
			panic(err)
		}
		lockfile, err := loadPoetryLock(os.DirFS("."))
		if err != nil {
			panic(err)
		}
		if len(args) == 0 {
			args = []string{"black", "flake8", "isort", "mypy"}
		}
		if problems := checkVersions(data, lockfile, args); len(problems) > 0 {
			for _, problem := range problems {
				fmt.Println(problem)
			}
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Read the .pre-commit-config.yaml file and return its contents.
func readPreCommitFile(fsys fs.FS) (file []byte, err error) {
	file, err = fs.ReadFile(fsys, ".pre-commit-config.yaml")
	return
}

type preCommitConfig struct {
	Repos []struct {
		Hooks []struct {
			Id                     string
			AdditionalDependencies []string `yaml:"additional_dependencies"`
		}
	}
}

// Parse the contents of a .pre-commit-config.yaml.
func loadPreCommitConfig(data []byte) (config preCommitConfig, err error) {
	err = yaml.Unmarshal(data, &config)
	return
}

type poetryLock struct {
	Metadata struct {
		LockVersion string `toml:"lock-version"`
	}
	Package []struct {
		Name    string
		Version string
	}
}

// Read and parse poetry.lock.
func loadPoetryLock(fsys fs.FS) (lockfile poetryLock, err error) {
	data, err := fs.ReadFile(fsys, "poetry.lock")
	if err != nil {
		return
	}
	err = toml.Unmarshal(data, &lockfile)
	return
}

// Check the versions of additional_dependencies in a pre-commit config against those in a poetry.lock.
// Only hooks with the specified `hookIds` will be checked.
//
// For each dependency in additional_dependencies, the following checks are made:
// - the dependency specifier must be in the format "package-name==exact.version"
// - the package name in the dependency specifier must be in the lockfile
// - the version in the dependency specifier must match the lockfile
func checkVersions(config preCommitConfig, lockfile poetryLock, hookIds []string) (problems []string) {
	for _, repo := range config.Repos {
		for _, hook := range repo.Hooks {
			if slices.Contains(hookIds, hook.Id) {
				for _, depspec := range hook.AdditionalDependencies {
					if problem := checkVersion(depspec, lockfile); problem != "" {
						problems = append(problems, fmt.Sprintf("%v: %v", depspec, problem))
					}
				}
			}
		}
	}
	return
}

// See PEP 508.
const identifierPat = `[a-zA-Z0-9](?:[-_.]*[a-zA-Z0-9])*`
const namePat = identifierPat

var extrasPat = fmt.Sprint(`\[\s*(?:`, identifierPat, `(?:\s*,\s*`, identifierPat, `)*)?\s*\]`)
var versionOnePat = `\s*(?:<|<=|!=|==|>=|>|~=|===)\s*(?:[a-zA-Z0-9]|[-_.*+!])+\s*`
var versionManyPat = fmt.Sprint(versionOnePat, `(?:\s*,`, versionOnePat, `)*`)
var versionspecPat = fmt.Sprint(`\(`, versionManyPat, `\)|`, versionManyPat)

var pat = regexp.MustCompile(fmt.Sprint(`^(`, namePat, `)\s*(?:`, extrasPat, `)?\s*(`, versionspecPat, `)?$`))

func checkVersion(depspec string, lockfile poetryLock) (problem string) {
	// Strictly speaking, the grammar of entries in additional_dependencies is defined by PEP 508; PEP 440 specifies
	// only the version constraints. However, in practice, it's easy enough to parse a minimal subset of PEP 508
	// specifiers given an existing PEP 440 parser. To simplify things, we reject specifiers with environment markers
	// (any string containing ";"). The other major problem is URLs (PEP 508 "urlspec"s) – we reject these too.
	depspec = strings.TrimSpace(depspec)
	if idx := strings.IndexAny(depspec, ";@"); idx != -1 {
		// since URLs can contain `;` and environment markers can contain `@`, we need to disambiguate
		if depspec[idx] == ';' {
			return "environment markers not permitted"
		} else {
			return "URLs not permitted"
		}
	}
	matches := pat.FindStringSubmatch(depspec)
	if matches == nil {
		return "invalid dependency specification"
	}
	lockedPackages := make(map[string]string)
	for _, pkg := range lockfile.Package {
		// TODO: normalise package names everywhere
		lockedPackages[pkg.Name] = pkg.Version
	}
	name, rawVersion := matches[1], matches[2]
	if rawVersion == "" {
		return "empty version spec not permitted"
	}
	versionSpec, err := version.NewSpecifiers(rawVersion)
	if err != nil {
		return "invalid version specification"
	}
	rawLockedVersion, ok := lockedPackages[name]
	if !ok {
		return "not found in poetry.lock"
	}
	lockedVersion, err := version.Parse(rawLockedVersion)
	if err != nil {
		panic(fmt.Sprintf("failed to parse version from poetry.lock: %q %q", name, rawLockedVersion))
	}
	if !versionSpec.Check(lockedVersion) {
		return fmt.Sprintf("version mismatch (expected: %v)", lockedVersion)
	}
	if !strings.Contains(rawVersion, "==") {
		return fmt.Sprintf("must specify an exact version (expected: %v==%v)", name, lockedVersion)
	}
	if strings.Contains(rawVersion, "===") {
		return fmt.Sprintf("arbitrary equality (===) not permitted (expected: %v==%v)", name, lockedVersion)
	}
	if strings.Contains(rawVersion, ".*") {
		return "trailing .* not permitted"
	}
	return ""
}
