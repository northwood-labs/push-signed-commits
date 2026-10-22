// Copyright 2026, Northwood Labs, LLC <license@northwood-labs.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"errors"
	"testing"

	"github.com/go-openapi/testify/assert"
	"github.com/go-openapi/testify/require"
)

func TestValidateBranchName(t *testing.T) {
	testCases := []struct {
		name        string
		input       *BranchNameInput
		errContains []string
		wantErr     bool
	}{
		{
			name: "local name with origin/ prefix is rejected",
			input: &BranchNameInput{
				BranchName: "origin/feature",
				BranchType: "local",
				RemoteName: "origin",
			},
			wantErr:     true,
			errContains: []string{"origin/", "local"},
		},
		{
			name: "local name with refs/heads/ prefix is rejected",
			input: &BranchNameInput{
				BranchName: "refs/heads/feature",
				BranchType: "local",
				RemoteName: "origin",
			},
			wantErr:     true,
			errContains: []string{"refs/heads/", "local"},
		},
		{
			name: "local name with remote-name prefix is rejected",
			input: &BranchNameInput{
				BranchName: "upstream/feature",
				BranchType: "local",
				RemoteName: "upstream",
			},
			wantErr:     true,
			errContains: []string{"remote name", "local"},
		},
		{
			name: "remote name with origin/ prefix is rejected",
			input: &BranchNameInput{
				BranchName: "origin/main",
				BranchType: "remote",
				RemoteName: "origin",
			},
			wantErr:     true,
			errContains: []string{"origin/", "remote"},
		},
		{
			name: "remote name with refs/heads/ prefix is rejected",
			input: &BranchNameInput{
				BranchName: "refs/heads/main",
				BranchType: "remote",
				RemoteName: "origin",
			},
			wantErr:     true,
			errContains: []string{"refs/heads/", "remote"},
		},
		{
			name: "remote name with remote-name prefix is rejected",
			input: &BranchNameInput{
				BranchName: "upstream/main",
				BranchType: "remote",
				RemoteName: "upstream",
			},
			wantErr:     true,
			errContains: []string{"remote name", "remote"},
		},
		{
			name: "clean local name passes",
			input: &BranchNameInput{
				BranchName: "feature/ABC-123",
				BranchType: "local",
				RemoteName: "origin",
			},
			wantErr: false,
		},
		{
			name: "clean remote name passes",
			input: &BranchNameInput{
				BranchName: "main",
				BranchType: "remote",
				RemoteName: "origin",
			},
			wantErr: false,
		},
		{
			name: "name embedding origin without the slash prefix passes",
			input: &BranchNameInput{
				BranchName: "feature-origin/work",
				BranchType: "local",
				RemoteName: "origin",
			},
			wantErr: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateBranchName(testCase.input)

			if !testCase.wantErr {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.True(
				t,
				errors.Is(err, ErrInvalidBranchName),
				"expected error to wrap ErrInvalidBranchName",
			)

			for _, fragment := range testCase.errContains {
				assert.Contains(t, err.Error(), fragment)
			}
		})
	}
}
