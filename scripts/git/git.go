package git

import (
	"fmt"
	"github.com/coreos/go-semver/semver"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"time"
)

var (
	sig = &object.Signature{
		Name:  "Version releaser",
		Email: "cyber@es.aau.dk",
		When:  time.Now(),
	}
)

type Repo struct {
	Repo git.Repository
}

func NewRepo(path string) (*Repo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return &Repo{
		Repo: *repo,
	}, nil
}

func (r Repo) Commit(version *semver.Version, files ...string) error {
	wt, err := r.Repo.Worktree()
	if err != nil {
		return err
	}
	for _, f := range files {
		if _, err := wt.Add(f); err != nil {
			return err
		}
	}

	commitMsg := fmt.Sprintf("Version bump (%s)", version.String())
	co := &git.CommitOptions{
		Author: sig,
	}
	wt.Commit(commitMsg, co)

	return nil
}

func (r Repo) Tag(version *semver.Version) error {
	headRef, err := r.Repo.Head()
	if err != nil {
		return err
	}

	cto := &git.CreateTagOptions{
		Message: version.String(),
		Tagger:  sig,
	}
	_, err = r.Repo.CreateTag(version.String(), headRef.Hash(), cto)
	return err
}

func (r Repo) CreateBranch(version *semver.Version) error {
	headRef, err := r.Repo.Head()
	if err != nil {
		return err
	}
	ref := plumbing.NewHashReference(referenceName(version.String()), headRef.Hash())
	return r.Repo.Storer.SetReference(ref)
}

func referenceName(branch string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch))
}
