package pkg

import triggersv1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"

type payloadDetails struct {
	PrNumber      int
	Sender        string
	Owner         string
	Repository    string
	DefaultBranch string
	SHA           string
	BaseBranch    string
}

type ownersConfig struct {
	Approvers []string `json:"approvers,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
}

type GitHubOwnersInterceptor struct {
	SecretRef              *triggersv1.SecretRef `json:"secretRef,omitempty"`
	OrgPublicMemberAllowed bool                  `json:"orgPublicMemberAllowed,omitempty"`
	RepoMemberAllowed      bool                  `json:"repoMemberAllowed,omitempty"`
}
