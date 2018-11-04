package git

import (
	"fmt"
	"github.com/coreos/go-semver/semver"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"time"
	"gopkg.in/src-d/go-git.v4/config"
	"regexp"
	"os"
	"os/user"
	"path"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

var (
	sig = &object.Signature{
		Name:  "Version releaser",
		Email: "cyber@es.aau.dk",
		When:  time.Now(),
	}
	refSpecRegex = "^refs/heads/(?P<Branch>.+)"
)

type InvalidRefSpecErr struct {
	name plumbing.ReferenceName
}

func (err InvalidRefSpecErr) Error() string {
	return fmt.Sprintf("Invalid reference name '%s'", err.name)
}

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
	ref := plumbing.NewHashReference(referenceName(version), headRef.Hash())
	return r.Repo.Storer.SetReference(ref)
}

func (r Repo) PushBranch() error {
	refSpec, err := r.headRefSpec()
	if err != nil {
		return err
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
		Auth: auth,
		RefSpecs: []config.RefSpec{
			refSpec,
		},
	}
	return r.Repo.Push(po)
}

func (r Repo) headRefSpec() (config.RefSpec, error) {
	headRef, err := r.Repo.Head()
	if err != nil {
		return "", err
	}

	matches := regexp.MustCompile(refSpecRegex).FindStringSubmatch(headRef.Name().String())
	if len(matches) < 2 {
		return "", InvalidRefSpecErr{headRef.Name()}
	}
	branch := matches[1]

	target := headRef.Target()
	if target == "" {
		fmt.Printf("Remote branch does not exist yet, create now\n")
		target = plumbing.ReferenceName(fmt.Sprintf("refs/remotes/origin/%s", branch))
	} else {
		fmt.Printf("Refspec target exists: %s", headRef.Target())
	}

	return config.RefSpec(fmt.Sprintf("+%s:%s", branch, target)), nil
}

func referenceName(version *semver.Version) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", version.String()))
}


