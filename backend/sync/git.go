package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

type GitRepo struct {
	repo     *git.Repository
	repoPath string
}

type SyncDirection int

const (
	SyncNone    SyncDirection = iota
	SyncPush
	SyncPull
	SyncConflict
)

// CloneOrOpen opens the repo at repoPath, or clones it from the given URL.
func CloneOrOpen(repoPath, repoURL, branch string, auth AuthType, token string) (*GitRepo, error) {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return nil, fmt.Errorf("build auth: %w", err)
	}

	repo, err := git.PlainOpen(repoPath)
	if err == nil {
		return &GitRepo{repo: repo, repoPath: repoPath}, nil
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return nil, fmt.Errorf("create parent dir: %w", err)
	}

	refName := plumbing.NewBranchReferenceName(branch)
	repo, err = git.PlainClone(repoPath, false, &git.CloneOptions{
		URL:           repoURL,
		Auth:          authMethod,
		ReferenceName: refName,
		SingleBranch:  true,
	})
	if err != nil {
		if errors.Is(err, gittransport.ErrEmptyRemoteRepository) {
			return initEmpty(repoPath, repoURL)
		}
		return nil, fmt.Errorf("clone: %w", err)
	}

	return &GitRepo{repo: repo, repoPath: repoPath}, nil
}

func initEmpty(repoPath, repoURL string) (*GitRepo, error) {
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	}); err != nil {
		return nil, fmt.Errorf("create remote: %w", err)
	}
	return &GitRepo{repo: repo, repoPath: repoPath}, nil
}

// StageAndCommit stages all files and creates a commit. Returns true if committed.
func (g *GitRepo) StageAndCommit(msg string) (bool, error) {
	wt, err := g.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("status: %w", err)
	}
	if status.IsClean() {
		return false, nil
	}

	if _, err := wt.Add("."); err != nil {
		return false, fmt.Errorf("add: %w", err)
	}

	_, err = wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "uniTerm",
			Email: "uniterm@local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

func (g *GitRepo) Push(auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return err
	}
	return g.repo.Push(&git.PushOptions{Auth: authMethod})
}

func (g *GitRepo) Pull(auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return err
	}
	wt, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	return wt.Pull(&git.PullOptions{Auth: authMethod, SingleBranch: true})
}

func (g *GitRepo) Fetch(auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return err
	}
	return g.repo.Fetch(&git.FetchOptions{Auth: authMethod, Force: true})
}

// CompareHeads returns sync direction after fetching.
func (g *GitRepo) CompareHeads(branch string) (SyncDirection, *time.Time, *time.Time, error) {
	localRef, err := g.repo.Head()
	if err != nil {
		return SyncNone, nil, nil, fmt.Errorf("local head: %w", err)
	}
	localHash := localRef.Hash()

	remoteRef, err := g.repo.Reference(
		plumbing.NewRemoteReferenceName("origin", branch), true,
	)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return SyncPush, nil, nil, nil
		}
		return SyncNone, nil, nil, fmt.Errorf("remote ref: %w", err)
	}
	remoteHash := remoteRef.Hash()

	if localHash == remoteHash {
		return SyncNone, nil, nil, nil
	}

	localCommit, err := g.repo.CommitObject(localHash)
	if err != nil {
		return SyncNone, nil, nil, fmt.Errorf("local commit: %w", err)
	}
	remoteCommit, err := g.repo.CommitObject(remoteHash)
	if err != nil {
		return SyncNone, nil, nil, fmt.Errorf("remote commit: %w", err)
	}

	localTime := localCommit.Committer.When
	remoteTime := remoteCommit.Committer.When

	localAncestor, _ := localCommit.IsAncestor(remoteCommit)
	remoteAncestor, _ := remoteCommit.IsAncestor(localCommit)

	if remoteAncestor {
		return SyncPush, &localTime, &remoteTime, nil
	}
	if localAncestor {
		return SyncPull, &localTime, &remoteTime, nil
	}
	return SyncConflict, &localTime, &remoteTime, nil
}

// ForcePush pushes with force, overwriting remote.
func (g *GitRepo) ForcePush(auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return err
	}
	return g.repo.Push(&git.PushOptions{Auth: authMethod, Force: true})
}

// ResetToRemote resets local HEAD to match remote branch.
func (g *GitRepo) ResetToRemote(branch string) error {
	wt, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	remoteRef, err := g.repo.Reference(
		plumbing.NewRemoteReferenceName("origin", branch), true,
	)
	if err != nil {
		return fmt.Errorf("remote ref: %w", err)
	}
	return wt.Reset(&git.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   git.HardReset,
	})
}

// TestConnection verifies the repo URL is reachable.
func TestConnection(repoURL string, auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	remote := git.NewRemote(nil, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})
	_, err = remote.List(&git.ListOptions{Auth: authMethod})
	if err != nil {
		return fmt.Errorf("remote unreachable: %w", err)
	}
	return nil
}

func buildAuth(auth AuthType, token string) (gittransport.AuthMethod, error) {
	switch auth {
	case AuthTypeSSH:
		return buildSSHAuth()
	case AuthTypeToken:
		return &githttp.BasicAuth{
			Username: "token",
			Password: token,
		}, nil
	default:
		return nil, fmt.Errorf("unknown auth type: %s", auth)
	}
}

func buildSSHAuth() (*gitssh.PublicKeys, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}
	sshDir := filepath.Join(home, ".ssh")

	keyNames := []string{"id_ed25519", "id_rsa", "id_ecdsa"}
	for _, name := range keyNames {
		keyPath := filepath.Join(sshDir, name)
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			continue
		}
		signer, err := cryptossh.ParsePrivateKey(keyData)
		if err != nil {
			continue
		}
		return &gitssh.PublicKeys{User: "git", Signer: signer}, nil
	}
	return nil, fmt.Errorf("no SSH private key found in %s", sshDir)
}
