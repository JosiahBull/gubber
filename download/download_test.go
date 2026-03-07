package download

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()

	existingFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	if !Exists(existingFile) {
		t.Error("Exists() returned false for existing file")
	}
	if Exists(filepath.Join(tmpDir, "nope.txt")) {
		t.Error("Exists() returned true for non-existing file")
	}
	if !Exists(tmpDir) {
		t.Error("Exists() returned false for existing directory")
	}
}

func TestCopy(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")
	content := []byte("file content for copy test")

	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := Copy(src, dst); err != nil {
		t.Fatalf("Copy() error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("Copy() content = %q, want %q", got, content)
	}
}

func TestCopy_SrcNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	err := Copy(filepath.Join(tmpDir, "nope"), filepath.Join(tmpDir, "dst"))
	if err == nil {
		t.Fatal("expected error copying non-existent source")
	}
}

func TestCopySymLink(t *testing.T) {
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0644); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(tmpDir, "link")
	if err := os.Symlink(target, src); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmpDir, "link_copy")
	if err := CopySymLink(src, dst); err != nil {
		t.Fatalf("CopySymLink() error: %v", err)
	}

	got, err := os.Readlink(dst)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if got != target {
		t.Errorf("CopySymLink() target = %q, want %q", got, target)
	}
}

func TestCreateIfNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	newDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := CreateIfNotExists(newDir, 0755); err != nil {
		t.Fatalf("CreateIfNotExists() error: %v", err)
	}
	if !Exists(newDir) {
		t.Error("directory was not created")
	}

	// idempotent
	if err := CreateIfNotExists(newDir, 0755); err != nil {
		t.Fatalf("CreateIfNotExists() second call error: %v", err)
	}
}

func TestCopyDirectory(t *testing.T) {
	srcDir := t.TempDir()

	// create structure: srcDir/sub/file.txt and srcDir/root.txt
	sub := filepath.Join(srcDir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file.txt"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(t.TempDir(), "dest")
	if err := os.Mkdir(dstDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := CopyDirectory(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDirectory() error: %v", err)
	}

	// verify root.txt
	got, err := os.ReadFile(filepath.Join(dstDir, "root.txt"))
	if err != nil {
		t.Fatalf("root.txt not copied: %v", err)
	}
	if string(got) != "root" {
		t.Errorf("root.txt content = %q, want %q", got, "root")
	}

	// verify sub/file.txt
	got, err = os.ReadFile(filepath.Join(dstDir, "sub", "file.txt"))
	if err != nil {
		t.Fatalf("sub/file.txt not copied: %v", err)
	}
	if string(got) != "nested" {
		t.Errorf("sub/file.txt content = %q, want %q", got, "nested")
	}
}

func TestMoveFolder(t *testing.T) {
	baseDir := t.TempDir()

	srcDir := filepath.Join(baseDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("moved"), 0644); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(baseDir, "dst")
	if err := os.Mkdir(dstDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := MoveFolder(srcDir, dstDir); err != nil {
		t.Fatalf("MoveFolder() error: %v", err)
	}

	if Exists(srcDir) {
		t.Error("source directory still exists after move")
	}

	got, err := os.ReadFile(filepath.Join(dstDir, "data.txt"))
	if err != nil {
		t.Fatalf("data.txt not found in dest: %v", err)
	}
	if string(got) != "moved" {
		t.Errorf("data.txt content = %q, want %q", got, "moved")
	}
}

func TestMoveFolder_SrcNotExist(t *testing.T) {
	baseDir := t.TempDir()
	err := MoveFolder(filepath.Join(baseDir, "nope"), filepath.Join(baseDir, "dst"))
	if err == nil {
		t.Fatal("expected error moving non-existent source")
	}
}
