package git

import (
	"fmt"
	"github.com/coreos/go-semver/semver"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"os"
	"os/user"
	"path"
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
	ref := plumbing.NewHashReference(branchReferenceName(version), headRef.Hash())
	return r.Repo.Storer.SetReference(ref)
}

func (r Repo) Push(branches []*semver.Version, tags []*semver.Version) error {
	var refSpecs []config.RefSpec
	spec, err := r.refSpec(nil, "head")
	if err != nil {
		return err
	}
	refSpecs = append(refSpecs, spec)
	for _, b := range branches {
		spec, err := r.refSpec(b, "branch")
		if err != nil {
			return err
		}
		refSpecs = append(refSpecs, spec)
	}
	for _, t := range tags {
		spec, err := r.refSpec(t, "tag")
		if err != nil {
			return err
		}
		refSpecs = append(refSpecs, spec)
	}

	keyFile := os.Getenv("NTP_RELEASE_PEMFILE")
	if keyFile == "" {
		curUser, err := user.Current()
		if err != nil {
			return err
		}
		fmt.Println("Environment variable 'NTP_RELEASE_PEMFILE' is not defined, using default '~/.id_rsa'")
		keyFile = path.Join(curUser.HomeDir, ".ssh", "id_rsa")
	}
	auth, err := ssh.NewPublicKeysFromFile("git", keyFile, "")
	if err != nil {
		return nil
	}

	po := &git.PushOptions{
		RemoteName: "origin",
		Auth:       auth,
		RefSpecs:   refSpecs,
	}
	return r.Repo.Push(po)
}

func (r Repo) refSpec(version *semver.Version, entityType string) (config.RefSpec, error) {
	var referenceName plumbing.ReferenceName
	switch entityType {
	case "branch":
		referenceName = branchReferenceName(version)
	case "tag":
		referenceName = tagReferenceName(version)
	case "head":
		referenceName = plumbing.ReferenceName("HEAD")
	}

	fmt.Printf("Reference name: %s\n", referenceName)

	ref, err := r.Repo.Reference(referenceName, true)
	if err != nil {
		return "", err
	}

	remote := ref.Target()
	if remote == "" {
		remote = ref.Name()
	}

	return config.RefSpec(fmt.Sprintf("%s:%s", ref.Name(), remote)), nil

}

func branchReferenceName(version *semver.Version) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", version.String()))
}

func tagReferenceName(version *semver.Version) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/tags/%s", version.String()))
}
