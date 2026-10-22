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

// Package gitclientfake provides a hand-written fake implementation of the
// gitclient.GitClient interface for use in table-driven tests. It is not a
// mocking framework: each method returns values taken from table-defined
// result holders on the Fake and appends the call's arguments to a
// per-method recorded-call slice so tests can assert on what was invoked.
//
// The fake lives in its own non-test package so that it can be imported by
// the pusher package tests and the cmd package tests alike; a fake placed in
// a _test.go file would be visible only to the declaring package's tests.
package gitclientfake

import (
	"context"

	"go.nwlabs.dev/push-signed-commits/internal/gitclient"
)

// Compile-time assertion that Fake satisfies the gitclient.GitClient
// interface. This also keeps the gitclient.GitClient interface referenced,
// clearing any unused warning.
var _ gitclient.GitClient = (*Fake)(nil)

type (
	// Fake is a hand-written fake implementation of gitclient.GitClient.
	// Populate the *Result fields with the values each method should return
	// and inspect the *Calls slices afterward to assert on the arguments the
	// code under test passed. The zero value returns zero values and nil
	// errors for every method.
	Fake struct {
		RemoteBranchExistsResult   RemoteBranchExistsResult
		FetchRemoteBranchResult    FetchRemoteBranchResult
		RebaseOntoResult           RebaseOntoResult
		MergeBaseResult            MergeBaseResult
		RemoteHeadOIDResult        RemoteHeadOIDResult
		CommitsNotOnRemoteResult   CommitsNotOnRemoteResult
		CommitMessageBodyResult    CommitMessageBodyResult
		NameStatusResult           NameStatusResult
		FileContentsAtCommitResult FileContentsAtCommitResult

		RemoteBranchExistsCalls   []RemoteBranchExistsCall
		FetchRemoteBranchCalls    []FetchRemoteBranchCall
		RebaseOntoCalls           []RebaseOntoCall
		MergeBaseCalls            []MergeBaseCall
		RemoteHeadOIDCalls        []RemoteHeadOIDCall
		CommitsNotOnRemoteCalls   []CommitsNotOnRemoteCall
		CommitMessageBodyCalls    []CommitMessageBodyCall
		NameStatusCalls           []NameStatusCall
		FileContentsAtCommitCalls []FileContentsAtCommitCall
	}

	// RemoteBranchExistsResult holds the return values for
	// Fake.RemoteBranchExists.
	RemoteBranchExistsResult struct {
		Err    error
		Exists bool
	}

	// FetchRemoteBranchResult holds the return value for
	// Fake.FetchRemoteBranch.
	FetchRemoteBranchResult struct {
		Err error
	}

	// RebaseOntoResult holds the return value for Fake.RebaseOnto.
	RebaseOntoResult struct {
		Err error
	}

	// MergeBaseResult holds the return values for Fake.MergeBase.
	MergeBaseResult struct {
		Err error
		OID string
	}

	// RemoteHeadOIDResult holds the return values for Fake.RemoteHeadOID.
	RemoteHeadOIDResult struct {
		Err error
		OID string
	}

	// CommitsNotOnRemoteResult holds the return values for
	// Fake.CommitsNotOnRemote.
	CommitsNotOnRemoteResult struct {
		Err    error
		Hashes []string
	}

	// CommitMessageBodyResult holds the return values for
	// Fake.CommitMessageBody.
	CommitMessageBodyResult struct {
		Err  error
		Body string
	}

	// NameStatusResult holds the return values for Fake.NameStatus.
	NameStatusResult struct {
		Err   error
		Lines []string
	}

	// FileContentsAtCommitResult holds the return values for
	// Fake.FileContentsAtCommit.
	FileContentsAtCommitResult struct {
		Err      error
		Contents []byte
	}

	// RemoteBranchExistsCall records one invocation of
	// Fake.RemoteBranchExists.
	RemoteBranchExistsCall struct {
		RemoteName       string
		RemoteBranchName string
	}

	// FetchRemoteBranchCall records one invocation of
	// Fake.FetchRemoteBranch.
	FetchRemoteBranchCall struct {
		RemoteName       string
		RemoteBranchName string
	}

	// RebaseOntoCall records one invocation of Fake.RebaseOnto.
	RebaseOntoCall struct {
		Opts *gitclient.RebaseOptions
	}

	// MergeBaseCall records one invocation of Fake.MergeBase.
	MergeBaseCall struct {
		Opts *gitclient.MergeBaseOptions
	}

	// RemoteHeadOIDCall records one invocation of Fake.RemoteHeadOID.
	RemoteHeadOIDCall struct {
		RemoteName       string
		RemoteBranchName string
	}

	// CommitsNotOnRemoteCall records one invocation of
	// Fake.CommitsNotOnRemote.
	CommitsNotOnRemoteCall struct {
		Opts *gitclient.CommitRangeOptions
	}

	// CommitMessageBodyCall records one invocation of
	// Fake.CommitMessageBody.
	CommitMessageBodyCall struct {
		CommitHash string
	}

	// NameStatusCall records one invocation of Fake.NameStatus.
	NameStatusCall struct {
		CommitHash string
	}

	// FileContentsAtCommitCall records one invocation of
	// Fake.FileContentsAtCommit.
	FileContentsAtCommitCall struct {
		CommitHash string
		Path       string
	}
)

// RemoteBranchExists records its arguments and returns the table-defined
// result from RemoteBranchExistsResult.
func (f *Fake) RemoteBranchExists(
	_ context.Context,
	remoteName, remoteBranchName string,
) (bool, error) {
	f.RemoteBranchExistsCalls = append(f.RemoteBranchExistsCalls, RemoteBranchExistsCall{
		RemoteName:       remoteName,
		RemoteBranchName: remoteBranchName,
	})

	return f.RemoteBranchExistsResult.Exists, f.RemoteBranchExistsResult.Err
}

// FetchRemoteBranch records its arguments and returns the table-defined
// result from FetchRemoteBranchResult.
func (f *Fake) FetchRemoteBranch(
	_ context.Context,
	remoteName, remoteBranchName string,
) error {
	f.FetchRemoteBranchCalls = append(f.FetchRemoteBranchCalls, FetchRemoteBranchCall{
		RemoteName:       remoteName,
		RemoteBranchName: remoteBranchName,
	})

	return f.FetchRemoteBranchResult.Err
}

// RebaseOnto records its arguments and returns the table-defined result from
// RebaseOntoResult.
func (f *Fake) RebaseOnto(_ context.Context, opts *gitclient.RebaseOptions) error {
	f.RebaseOntoCalls = append(f.RebaseOntoCalls, RebaseOntoCall{Opts: opts})

	return f.RebaseOntoResult.Err
}

// MergeBase records its arguments and returns the table-defined result from
// MergeBaseResult.
func (f *Fake) MergeBase(_ context.Context, opts *gitclient.MergeBaseOptions) (string, error) {
	f.MergeBaseCalls = append(f.MergeBaseCalls, MergeBaseCall{Opts: opts})

	return f.MergeBaseResult.OID, f.MergeBaseResult.Err
}

// RemoteHeadOID records its arguments and returns the table-defined result
// from RemoteHeadOIDResult.
func (f *Fake) RemoteHeadOID(
	_ context.Context,
	remoteName, remoteBranchName string,
) (string, error) {
	f.RemoteHeadOIDCalls = append(f.RemoteHeadOIDCalls, RemoteHeadOIDCall{
		RemoteName:       remoteName,
		RemoteBranchName: remoteBranchName,
	})

	return f.RemoteHeadOIDResult.OID, f.RemoteHeadOIDResult.Err
}

// CommitsNotOnRemote records its arguments and returns the table-defined
// result from CommitsNotOnRemoteResult.
func (f *Fake) CommitsNotOnRemote(
	_ context.Context,
	opts *gitclient.CommitRangeOptions,
) ([]string, error) {
	f.CommitsNotOnRemoteCalls = append(f.CommitsNotOnRemoteCalls, CommitsNotOnRemoteCall{Opts: opts})

	return f.CommitsNotOnRemoteResult.Hashes, f.CommitsNotOnRemoteResult.Err
}

// CommitMessageBody records its arguments and returns the table-defined
// result from CommitMessageBodyResult.
func (f *Fake) CommitMessageBody(_ context.Context, commitHash string) (string, error) {
	f.CommitMessageBodyCalls = append(f.CommitMessageBodyCalls, CommitMessageBodyCall{
		CommitHash: commitHash,
	})

	return f.CommitMessageBodyResult.Body, f.CommitMessageBodyResult.Err
}

// NameStatus records its arguments and returns the table-defined result from
// NameStatusResult.
func (f *Fake) NameStatus(_ context.Context, commitHash string) ([]string, error) {
	f.NameStatusCalls = append(f.NameStatusCalls, NameStatusCall{CommitHash: commitHash})

	return f.NameStatusResult.Lines, f.NameStatusResult.Err
}

// FileContentsAtCommit records its arguments and returns the table-defined
// result from FileContentsAtCommitResult.
func (f *Fake) FileContentsAtCommit(
	_ context.Context,
	commitHash, path string,
) ([]byte, error) {
	f.FileContentsAtCommitCalls = append(f.FileContentsAtCommitCalls, FileContentsAtCommitCall{
		CommitHash: commitHash,
		Path:       path,
	})

	return f.FileContentsAtCommitResult.Contents, f.FileContentsAtCommitResult.Err
}
