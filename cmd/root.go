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
	"slices"

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
		_ = lockfile
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
					// TODO
					_ = depspec
				}
			}
		}
	}
	return
}
