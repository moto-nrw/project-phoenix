package architecture

import (
	"path"
	"strings"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

func readBasePolicyDocument(root, project, sha, filename string) ([]byte, error) {
	relative, err := repositoryRelativePath(root, project, filename)
	if err != nil {
		return nil, err
	}
	relative = strings.ReplaceAll(relative, "\\", "/")
	return artifacts.ReadPolicyAtRevision(root, sha, path.Dir(relative), path.Base(relative))
}
