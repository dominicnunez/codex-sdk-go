package protocol

import "testing"

func TestFilesystemNormalizationIsolation(t *testing.T) {
	for _, grant := range []bool{false, true} {
		subpath, depth := "../raw", uint64(9)
		location := &ProjectRootsFileSystemSpecialPath{Subpath: &subpath}
		path := &SpecialFileSystemPath{Value: FileSystemSpecialPathWrapper{Value: location}}
		entries := []FileSystemSandboxEntry{{Access: FileSystemAccessModeDeny, Path: FileSystemPathWrapper{Value: path}}}
		source := &AdditionalFileSystemPermissions{Entries: &entries, GlobScanMaxDepth: &depth, Read: []string{"../read"}, Write: []string{"../write"}}
		var clone *AdditionalFileSystemPermissions
		if grant {
			clone = normalizeGrantedPermissionProfileField(GrantedPermissionProfile{FileSystem: source}).FileSystem
		} else {
			clone = normalizeRequestPermissionProfileField(RequestPermissionProfile{FileSystem: source}).FileSystem
		}
		clonedLocation := (*clone.Entries)[0].Path.Value.(*SpecialFileSystemPath).Value.Value.(*ProjectRootsFileSystemSpecialPath)
		*clonedLocation.Subpath = "clone"
		*clone.GlobScanMaxDepth = 3
		(*clone.Entries)[0].Access = FileSystemAccessModeRead
		clone.Read[0], clone.Write[0] = "clone-read", "clone-write"
		if subpath != "../raw" || depth != 9 || entries[0].Access != FileSystemAccessModeDeny || source.Read[0] != "../read" || source.Write[0] != "../write" {
			t.Fatalf("grant=%v: clone mutated source: %+v", grant, source)
		}
		*location.Subpath, depth = "source", 12
		if *clonedLocation.Subpath != "clone" || *clone.GlobScanMaxDepth != 3 {
			t.Fatalf("grant=%v: source mutated clone", grant)
		}
	}
	entries := []FileSystemSandboxEntry{}
	clone := normalizeAdditionalFileSystemPermissionsField(&AdditionalFileSystemPermissions{Entries: &entries, Read: []string{"legacy"}})
	if clone.Entries == nil || *clone.Entries == nil || len(*clone.Entries) != 0 || clone.Read[0] != "legacy" {
		t.Fatal("explicit empty entries lost their precedence over legacy scope")
	}
}
