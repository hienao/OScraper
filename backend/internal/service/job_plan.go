package service

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"openlistscraper/internal/model"
)

type pathRewrite struct {
	from string
	to   string
}

func buildJobOperations(plan PreviewPlan) ([]model.ScrapeJobOperation, error) {
	operations := make([]model.ScrapeJobOperation, 0)
	sequence := 0
	appendOperation := func(operationType, source, target string, artifact int, kind string) {
		sequence++
		operations = append(operations, model.ScrapeJobOperation{
			Sequence: sequence, Type: operationType, SourcePath: source, TargetPath: target,
			Artifact: artifact, ArtifactKind: kind, Status: "pending",
		})
	}

	var rootRename *RenameItem
	nestedRenames := make([]RenameItem, 0)
	for index := range plan.ProposedDirectoryRenames {
		rename := plan.ProposedDirectoryRenames[index]
		if rename.SourcePath == plan.SourcePath {
			copy := rename
			rootRename = &copy
		} else {
			nestedRenames = append(nestedRenames, rename)
		}
	}
	currentRoot := plan.SourcePath
	finalRoot := plan.ProposedDirectoryPath
	mapFinalToCurrent := func(value string) string {
		if rootRename == nil || currentRoot == finalRoot {
			return value
		}
		return replacePathPrefix(value, finalRoot, currentRoot)
	}

	for _, directory := range plan.ProposedDirectoryCreates {
		appendOperation("mkdir", "", mapFinalToCurrent(directory), 0, "directory")
	}

	sort.Slice(nestedRenames, func(left, right int) bool {
		return pathDepth(nestedRenames[left].SourcePath) > pathDepth(nestedRenames[right].SourcePath)
	})
	rewrites := make([]pathRewrite, 0, len(nestedRenames))
	for _, rename := range nestedRenames {
		source := applyPathRewrites(rename.SourcePath, rewrites)
		target := mapFinalToCurrent(rename.TargetPath)
		appendPathMutation(&operations, &sequence, source, target, rename.AssetType)
		rewrites = append(rewrites, pathRewrite{from: rename.SourcePath, to: target})
	}

	for _, rename := range plan.ProposedFileRenames {
		source := applyPathRewrites(rename.SourcePath, rewrites)
		target := mapFinalToCurrent(rename.TargetPath)
		appendPathMutation(&operations, &sequence, source, target, rename.AssetType)
	}
	if rootRename != nil {
		appendPathMutation(&operations, &sequence, rootRename.SourcePath, rootRename.TargetPath, "directory")
	}
	for index, artifact := range plan.Artifacts {
		if artifact.Path == "" || (isNFOArtifact(artifact.Kind) && artifact.Content == "") || (!isNFOArtifact(artifact.Kind) && artifact.SourceURL == "") {
			return nil, fmt.Errorf("invalid %s artifact", artifact.Kind)
		}
		appendOperation("upload", "", artifact.Path, index+1, artifact.Kind)
	}
	return operations, nil
}

func appendPathMutation(operations *[]model.ScrapeJobOperation, sequence *int, source, target, assetType string) {
	if source == target {
		return
	}
	appendOperation := func(operationType, operationSource, operationTarget string) {
		*sequence = *sequence + 1
		*operations = append(*operations, model.ScrapeJobOperation{
			Sequence: *sequence, Type: operationType, SourcePath: operationSource, TargetPath: operationTarget,
			Artifact: 0, ArtifactKind: assetType, Status: "pending",
		})
	}
	if path.Dir(source) == path.Dir(target) {
		appendOperation("rename", source, target)
		return
	}
	movedPath := path.Join(path.Dir(target), path.Base(source))
	appendOperation("move", source, movedPath)
	if movedPath != target {
		appendOperation("rename", movedPath, target)
	}
}

func applyPathRewrites(value string, rewrites []pathRewrite) string {
	result := value
	for _, rewrite := range rewrites {
		result = replacePathPrefix(result, rewrite.from, rewrite.to)
	}
	return result
}

func replacePathPrefix(value, from, to string) string {
	if value == from {
		return to
	}
	if strings.HasPrefix(value, strings.TrimRight(from, "/")+"/") {
		return strings.TrimRight(to, "/") + strings.TrimPrefix(value, strings.TrimRight(from, "/"))
	}
	return value
}

func pathDepth(value string) int { return strings.Count(strings.Trim(value, "/"), "/") }
