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

import "testing"

// The root command should succeed.
func TestExecute(t *testing.T) {
	err := rootCmd.Execute()
	if err != nil {
		t.Fail()
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
