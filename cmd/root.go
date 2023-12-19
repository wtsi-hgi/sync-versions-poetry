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
		_ = data
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
