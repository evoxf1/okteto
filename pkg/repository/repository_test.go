// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package repository

import (
	"context"
	"net/url"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/stretchr/testify/assert"
)

type fakeRepositoryGetter struct {
	repository []*fakeRepository
	err        []error
	callCount  int
}

func (frg *fakeRepositoryGetter) get(_ string) (gitRepositoryInterface, error) {
	i := frg.callCount
	frg.callCount++
	if frg.err != nil && frg.err[i] != nil {
		return nil, frg.err[i]
	}
	return frg.repository[i], nil
}

type fakeRepository struct {
	worktree     *fakeWorktree
	head         *plumbing.Reference
	commit       *fakeCommit
	failInCommit bool
	err          error
}

func (fr fakeRepository) Worktree() (gitWorktreeInterface, error) {
	return fr.worktree, fr.err
}
func (fr fakeRepository) Head() (*plumbing.Reference, error) {
	if fr.failInCommit {
		return fr.head, nil
	}
	return fr.head, fr.err
}

func (fr fakeRepository) CommitObject(plumbing.Hash) (gitCommitInterface, error) {
	return fr.commit, fr.err
}

type fakeWorktree struct {
	status oktetoGitStatus
	root   string
	err    error
}

func (fw fakeWorktree) GetRoot() string {
	return fw.root
}

func (fw fakeWorktree) Status(context.Context, LocalGitInterface) (oktetoGitStatus, error) {
	return fw.status, fw.err
}

type fakeCommit struct {
	tree *object.Tree
	err  error
}

func (fc *fakeCommit) Tree() (*object.Tree, error) {
	return fc.tree, fc.err
}

func TestNewRepo(t *testing.T) {
	tt := []struct {
		name            string
		GitCommit       string
		remoteDeploy    string
		expectedControl repositoryInterface
	}{
		{
			name:      "GitCommit is empty",
			GitCommit: "",
			expectedControl: gitRepoController{
				repoGetter: gitRepositoryGetter{},
			},
		},
		{
			name:      "GitCommit is not empty",
			GitCommit: "1234567890",
			expectedControl: gitRepoController{
				repoGetter: gitRepositoryGetter{},
			},
		},
		{
			name:         "GitCommit is not empty in remote deploy",
			GitCommit:    "1234567890",
			remoteDeploy: "true",
			expectedControl: oktetoRemoteRepoController{
				gitCommit: "1234567890",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(constants.OktetoGitCommitEnvVar, tc.GitCommit)
			t.Setenv(constants.OktetoDeployRemote, string(tc.remoteDeploy))
			r := NewRepository("https://my-repo/okteto/okteto")
			assert.Equal(t, "/okteto/okteto", r.url.Path)
			assert.IsType(t, tc.expectedControl, r.control)
		})
	}
}

func TestIsEqual(t *testing.T) {
	type input struct {
		r Repository
		o Repository
	}
	var tests = []struct {
		name     string
		input    input
		expected bool
	}{
		{
			name: "r is nil -> false",
			input: input{
				r: Repository{},
				o: Repository{url: &repositoryURL{}},
			},
			expected: false,
		},
		{
			name: "o is nil -> false",
			input: input{
				r: Repository{url: &repositoryURL{}},
				o: Repository{},
			},
			expected: false,
		},
		{
			name: "r and o are nil -> false",
			input: input{
				r: Repository{},
				o: Repository{},
			},
			expected: false,
		},
		{
			name: "different hostname -> false",
			input: input{
				r: Repository{url: &repositoryURL{url.URL{Host: "my-hub"}}},
				o: Repository{url: &repositoryURL{url.URL{Host: "my-hub2"}}},
			},
			expected: false,
		},
		{
			name: "different path -> false",
			input: input{
				r: Repository{url: &repositoryURL{url.URL{Host: "my-hub", Path: "okteto/repo1"}}},
				o: Repository{url: &repositoryURL{url.URL{Host: "my-hub", Path: "okteto/repo2"}}},
			},
			expected: false,
		},
		{
			name: "equal -> true",
			input: input{
				r: Repository{url: &repositoryURL{url.URL{Host: "my-hub", Path: "okteto/repo1"}}},
				o: Repository{url: &repositoryURL{url.URL{Host: "my-hub", Path: "okteto/repo2"}}},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.r.IsEqual(tt.input.o)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanPath(t *testing.T) {
	var tests = []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path starts with /",
			input:    "/okteto/okteto",
			expected: "okteto/okteto",
		},
		{
			name:     "path ends with .git",
			input:    "okteto/okteto.git",
			expected: "okteto/okteto",
		},
		{
			name:     "path starts with / and ends with .git",
			input:    "/okteto/okteto.git",
			expected: "okteto/okteto",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_GetAnonymizedRepo(t *testing.T) {
	tests := []struct {
		name       string
		repository *Repository
		expected   string
	}{
		{
			name: "https repo without credentials",
			repository: &Repository{
				url: &repositoryURL{
					url.URL{
						Scheme: "https",
						Host:   "github.com",
						Path:   "/okteto/okteto",
					},
				},
			},
			expected: "https://github.com/okteto/okteto",
		},
		{
			name: "ssh repo",
			repository: &Repository{
				url: &repositoryURL{
					url.URL{
						Scheme: "ssh",
						Host:   "github.com",
						Path:   "okteto/okteto.git",
						User:   url.User("git"),
					},
				}},
			expected: "ssh://github.com/okteto/okteto.git",
		},
		{
			name: "https repo with credentials",
			repository: &Repository{
				url: &repositoryURL{
					url.URL{
						Scheme: "https",
						Host:   "github.com",
						Path:   "/okteto/okteto",
						User:   url.UserPassword("git", "PASSWORD"),
					},
				}},
			expected: "https://github.com/okteto/okteto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.repository.GetAnonymizedRepo()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_getURLFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected repositoryURL
	}{
		{
			name: "https repo without credentials",
			path: "https://github.com/okteto/okteto",
			expected: repositoryURL{
				url.URL{
					Scheme: "https",
					Host:   "github.com",
					Path:   "/okteto/okteto",
				},
			},
		},
		{
			name: "ssh repo",
			path: "git@github.com:okteto/okteto.git",
			expected: repositoryURL{
				url.URL{
					Scheme: "ssh",
					Host:   "github.com",
					Path:   "okteto/okteto.git",
					User:   url.User("git"),
				},
			},
		},
		{
			name: "https repo with credentials",
			path: "https://git:PASSWORD@github.com/okteto/okteto",
			expected: repositoryURL{
				url.URL{
					Scheme: "https",
					Host:   "github.com",
					Path:   "/okteto/okteto",
					User:   url.UserPassword("git", "PASSWORD"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getURLFromPath(tt.path)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_String(t *testing.T) {
	r := &repositoryURL{
		url.URL{
			Scheme: "http",
			Host:   "okteto.com",
			Path:   "docs",
			User:   url.UserPassword("test", "password"),
		},
	}

	expected := "http://okteto.com/docs"
	got := r.String()

	assert.Equal(t, expected, got)
	assert.NotNil(t, r.URL.User)
}
