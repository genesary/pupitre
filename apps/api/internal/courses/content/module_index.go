package content

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
)

// FetchModuleIndex fetches and parses a module index YAML file from git.
// Index entries that omit src or ref inherit them from the parent module.
func FetchModuleIndex(ctx context.Context, gc *GitCache, parent Module, token string) ([]Module, error) {
	data, err := gc.FetchModuleContent(ctx, parent.Src, parent.Ref, parent.Path, token)
	if err != nil {
		return nil, err
	}

	var entries []ModuleIndexEntry

	err = yaml.Unmarshal(data, &entries)
	if err != nil {
		return nil, fmt.Errorf("parse module index: %w", err)
	}

	modules := make([]Module, 0, len(entries))
	for _, entry := range entries {
		src, ref, typ := entry.Src, entry.Ref, entry.Type
		if src == "" {
			src = parent.Src
		}

		if ref == "" {
			ref = parent.Ref
		}

		if typ == "" {
			typ = ModuleTypeText
		}

		modules = append(modules, Module{
			Name:          entry.Name,
			Type:          typ,
			Src:           src,
			Ref:           ref,
			Path:          entry.Path,
			Hidden:        entry.Hidden,
			Prerequisites: entry.Prerequisites,
		})
	}

	return modules, nil
}
