package remote

import (
	. "github.com/yourbase/yb/types"
)

type RemoteBuild struct {
	PatchContents []byte
	PatchPath     string
	WorkDir       string
	TargetName    string
	BranchName    string
	NoAccel       bool
	DisableAccels []bool
	PublicRepo    bool
	Remotes       []GitRemote
	remoteBranch  string
	baseCommit    string
	goGit         bool
	onlyCommitted bool
	commitCount   int64
}

func NewRemoteBuild() *RemoteBuild {
	return new(RemoteBuild)
}
