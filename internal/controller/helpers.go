package controller

import (
	"fmt"
	"sort"
)

func matchRoutePath(paths []RoutePath, matchPath string) *RoutePath {
	for _, path := range paths {
		if path.Path == matchPath {
			return &path
		}
	}
	return nil
}

func deleteRoutePath(paths []RoutePath, matchPath string) []RoutePath {
	for i, path := range paths {
		if path.Path == matchPath {
			return append(paths[:i], paths[i+1:]...)
		}
	}
	return paths
}

func sortRulePathsLength(routes []RoutePath) {
	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].Path) > len(routes[j].Path)
	})
}

func namespaceFormat(namespace string, resourceName string) string {
	return fmt.Sprintf("%s/%s", namespace, resourceName)
}
