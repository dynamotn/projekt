// Package lazypath provides configuration management for project folders.
//
// It handles loading, saving, and validating folder configurations from YAML files.
// Folders can be regular project folders or workspaces containing multiple projects.
//
// Example usage:
//
//	lazypath.InitConfig()
//	config := lazypath.GetConfig()
//
//	folder := &lazypath.Folder{
//	    Path:        "/home/user/projects/myapp",
//	    Prefix:      "app",
//	    IsWorkspace: false,
//	}
//	err := folder.AddToConfig()
package lazypath
