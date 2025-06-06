package gptscript

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestWorkspaceIDRequiredForDelete(t *testing.T) {
	if err := g.DeleteWorkspace(context.Background(), ""); err == nil {
		t.Error("Expected error but got nil")
	}
}

func TestCreateAndDeleteWorkspace(t *testing.T) {
	id, err := g.CreateWorkspace(context.Background(), "directory")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	err = g.DeleteWorkspace(context.Background(), id)
	if err != nil {
		t.Errorf("Error deleting workspace: %v", err)
	}
}

func TestCreateAndDeleteWorkspaceFromWorkspace(t *testing.T) {
	id, err := g.CreateWorkspace(context.Background(), "directory")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err = g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "file.txt", []byte("hello world"), WriteFileInWorkspaceOptions{
		WorkspaceID: id,
	})
	if err != nil {
		t.Errorf("Error creating file: %v", err)
	}

	newID, err := g.CreateWorkspace(context.Background(), "directory", id)
	if err != nil {
		t.Errorf("Error creating workspace from workspace: %v", err)
	}

	content, err := g.ReadFileInWorkspace(context.Background(), "file.txt", ReadFileInWorkspaceOptions{
		WorkspaceID: newID,
	})
	if err != nil {
		t.Fatalf("Error reading file: %v", err)
	}

	if !bytes.Equal(content, []byte("hello world")) {
		t.Errorf("Unexpected content: %s", content)
	}

	err = g.DeleteWorkspace(context.Background(), id)
	if err != nil {
		t.Errorf("Error deleting workspace: %v", err)
	}
}

func TestWriteReadAndDeleteFileFromWorkspace(t *testing.T) {
	id, err := g.CreateWorkspace(context.Background(), "directory")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	content, err := g.ReadFileInWorkspace(context.Background(), "test.txt", ReadFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if !bytes.Equal(content, []byte("test")) {
		t.Errorf("Unexpected content: %s", content)
	}

	// Read the file and request the revision ID
	contentWithRevision, err := g.ReadFileWithRevisionInWorkspace(context.Background(), "test.txt", ReadFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if !bytes.Equal(contentWithRevision.Content, []byte("test")) {
		t.Errorf("Unexpected content: %s", contentWithRevision.Content)
	}

	if contentWithRevision.RevisionID == "" {
		t.Errorf("Expected file revision ID when requesting it: %s", contentWithRevision.RevisionID)
	}

	// Stat the file to ensure it exists
	fileInfo, err := g.StatFileInWorkspace(context.Background(), "test.txt", StatFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error statting file: %v", err)
	}

	if fileInfo.WorkspaceID != id {
		t.Errorf("Unexpected file workspace ID: %v", fileInfo.WorkspaceID)
	}

	if fileInfo.Name != "test.txt" {
		t.Errorf("Unexpected file name: %s", fileInfo.Name)
	}

	if fileInfo.Size != 4 {
		t.Errorf("Unexpected file size: %d", fileInfo.Size)
	}

	if fileInfo.ModTime.IsZero() {
		t.Errorf("Unexpected file mod time: %v", fileInfo.ModTime)
	}

	if fileInfo.MimeType != "text/plain" {
		t.Errorf("Unexpected file mime type: %s", fileInfo.MimeType)
	}

	if fileInfo.RevisionID != "" {
		t.Errorf("Unexpected file revision ID when not requesting it: %s", fileInfo.RevisionID)
	}

	// Stat file and request the revision ID
	fileInfo, err = g.StatFileInWorkspace(context.Background(), "test.txt", StatFileInWorkspaceOptions{WorkspaceID: id, WithLatestRevisionID: true})
	if err != nil {
		t.Errorf("Error statting file: %v", err)
	}

	if fileInfo.WorkspaceID != id {
		t.Errorf("Unexpected file workspace ID: %v", fileInfo.WorkspaceID)
	}

	if fileInfo.RevisionID == "" {
		t.Errorf("Expected file revision ID when requesting it: %s", fileInfo.RevisionID)
	}

	// Ensure we get the error we expect when trying to read a non-existent file
	_, err = g.ReadFileInWorkspace(context.Background(), "test1.txt", ReadFileInWorkspaceOptions{WorkspaceID: id})
	if nf := (*NotFoundInWorkspaceError)(nil); !errors.As(err, &nf) {
		t.Errorf("Unexpected error reading non-existent file: %v", err)
	}

	err = g.DeleteFileInWorkspace(context.Background(), "test.txt", DeleteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting file: %v", err)
	}
}

func TestRevisionsForFileInWorkspace(t *testing.T) {
	id, err := g.CreateWorkspace(context.Background(), "directory")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test0"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test1"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test2"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	revisions, err := g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 2 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	for i, rev := range revisions {
		if rev.WorkspaceID != id {
			t.Errorf("Unexpected file workspace ID: %v", rev.WorkspaceID)
		}

		if rev.Name != "test.txt" {
			t.Errorf("Unexpected file name: %s", rev.Name)
		}

		if rev.Size != 5 {
			t.Errorf("Unexpected file size: %d", rev.Size)
		}

		if rev.ModTime.IsZero() {
			t.Errorf("Unexpected file mod time: %v", rev.ModTime)
		}

		if rev.MimeType != "text/plain" {
			t.Errorf("Unexpected file mime type: %s", rev.MimeType)
		}

		if rev.RevisionID != fmt.Sprintf("%d", i+1) {
			t.Errorf("Unexpected revision ID: %s", rev.RevisionID)
		}
	}

	err = g.DeleteRevisionForFileInWorkspace(context.Background(), "test.txt", "1", DeleteRevisionForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting revision for file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 1 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	err = g.DeleteFileInWorkspace(context.Background(), "test.txt", DeleteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 0 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}
}

func TestDisableCreateRevisionsForFileInWorkspace(t *testing.T) {
	id, err := g.CreateWorkspace(context.Background(), "directory")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test0"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test1"), WriteFileInWorkspaceOptions{WorkspaceID: id, CreateRevision: new(bool)})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test2"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	revisions, err := g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 1 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	for i, rev := range revisions {
		if rev.WorkspaceID != id {
			t.Errorf("Unexpected file workspace ID: %v", rev.WorkspaceID)
		}

		if rev.Name != "test.txt" {
			t.Errorf("Unexpected file name: %s", rev.Name)
		}

		if rev.Size != 5 {
			t.Errorf("Unexpected file size: %d", rev.Size)
		}

		if rev.ModTime.IsZero() {
			t.Errorf("Unexpected file mod time: %v", rev.ModTime)
		}

		if rev.MimeType != "text/plain" {
			t.Errorf("Unexpected file mime type: %s", rev.MimeType)
		}

		if rev.RevisionID != fmt.Sprintf("%d", i+1) {
			t.Errorf("Unexpected revision ID: %s", rev.RevisionID)
		}
	}

	err = g.DeleteRevisionForFileInWorkspace(context.Background(), "test.txt", "1", DeleteRevisionForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting revision for file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 0 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	err = g.DeleteFileInWorkspace(context.Background(), "test.txt", DeleteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting file: %v", err)
	}
}

func TestConflictsForFileInWorkspace(t *testing.T) {
	id, err := g.CreateWorkspace(context.Background(), "directory")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	ce := (*ConflictInWorkspaceError)(nil)
	// Writing a new file with a non-zero latest revision should fail
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test0"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: "1"})
	if err == nil || !errors.As(err, &ce) {
		t.Errorf("Expected error writing file with non-zero latest revision: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test0"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test1"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	revisions, err := g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 1 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	// Writing to the file with the latest revision should succeed
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test2"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: revisions[0].RevisionID})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 2 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	// Writing to the file with the same revision should fail
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test3"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: revisions[0].RevisionID})
	if err == nil || !errors.As(err, &ce) {
		t.Errorf("Expected error writing file with same revision: %v", err)
	}

	latestRevisionID := revisions[1].RevisionID
	err = g.DeleteRevisionForFileInWorkspace(context.Background(), "test.txt", latestRevisionID, DeleteRevisionForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting revision for file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 1 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	// Ensure we cannot write a new file with the zero-th revision ID
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test4"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: revisions[0].RevisionID})
	if err == nil || !errors.As(err, &ce) {
		t.Errorf("Unexpected error writing to file: %v", err)
	}

	// Ensure we can write a new file after deleting the latest revision
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test4"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: latestRevisionID})
	if err != nil {
		t.Errorf("Error writing file: %v", err)
	}

	err = g.DeleteFileInWorkspace(context.Background(), "test.txt", DeleteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting file: %v", err)
	}
}

func TestLsComplexWorkspace(t *testing.T) {
	id, err := g.CreateWorkspace(context.Background(), "directory")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "test/test1.txt", []byte("hello1"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test1/test2.txt", []byte("hello2"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test1/test3.txt", []byte("hello3"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), ".hidden.txt", []byte("hidden"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating hidden file: %v", err)
	}

	// List all files
	content, err := g.ListFilesInWorkspace(context.Background(), ListFilesInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error listing files: %v", err)
	}

	if len(content) != 4 {
		t.Errorf("Unexpected number of files: %d", len(content))
	}

	// List files in subdirectory
	content, err = g.ListFilesInWorkspace(context.Background(), ListFilesInWorkspaceOptions{WorkspaceID: id, Prefix: "test1"})
	if err != nil {
		t.Fatalf("Error listing files: %v", err)
	}

	if len(content) != 2 {
		t.Errorf("Unexpected number of files: %d", len(content))
	}

	// Remove all files with test1 prefix
	err = g.RemoveAll(context.Background(), RemoveAllOptions{WorkspaceID: id, WithPrefix: "test1"})
	if err != nil {
		t.Fatalf("Error removing files: %v", err)
	}

	// List files in subdirectory
	content, err = g.ListFilesInWorkspace(context.Background(), ListFilesInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error listing files: %v", err)
	}

	if len(content) != 2 {
		t.Errorf("Unexpected number of files: %d", len(content))
	}
}

func TestCreateAndDeleteWorkspaceS3(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" || os.Getenv("WORKSPACE_PROVIDER_S3_BUCKET") == "" {
		t.Skip("Skipping test because AWS credentials are not set")
	}

	id, err := g.CreateWorkspace(context.Background(), "s3")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	err = g.DeleteWorkspace(context.Background(), id)
	if err != nil {
		t.Errorf("Error deleting workspace: %v", err)
	}
}

func TestCreateAndDeleteWorkspaceFromWorkspaceS3(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" || os.Getenv("WORKSPACE_PROVIDER_S3_BUCKET") == "" {
		t.Skip("Skipping test because AWS credentials are not set")
	}

	id, err := g.CreateWorkspace(context.Background(), "s3")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "file.txt", []byte("hello world"), WriteFileInWorkspaceOptions{
		WorkspaceID: id,
	})
	if err != nil {
		t.Errorf("Error creating file: %v", err)
	}

	newID, err := g.CreateWorkspace(context.Background(), "s3", id)
	if err != nil {
		t.Errorf("Error creating workspace from workspace: %v", err)
	}

	content, err := g.ReadFileInWorkspace(context.Background(), "file.txt", ReadFileInWorkspaceOptions{
		WorkspaceID: newID,
	})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if !bytes.Equal(content, []byte("hello world")) {
		t.Errorf("Unexpected content: %s", content)
	}

	err = g.DeleteWorkspace(context.Background(), id)
	if err != nil {
		t.Errorf("Error deleting workspace: %v", err)
	}

	err = g.DeleteWorkspace(context.Background(), newID)
	if err != nil {
		t.Errorf("Error deleting new workspace: %v", err)
	}
}

func TestCreateAndDeleteDirectoryWorkspaceFromWorkspaceS3(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" || os.Getenv("WORKSPACE_PROVIDER_S3_BUCKET") == "" {
		t.Skip("Skipping test because AWS credentials are not set")
	}

	id, err := g.CreateWorkspace(context.Background(), "s3")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "file.txt", []byte("hello world"), WriteFileInWorkspaceOptions{
		WorkspaceID: id,
	})
	if err != nil {
		t.Errorf("Error creating file: %v", err)
	}

	newID, err := g.CreateWorkspace(context.Background(), "directory", id)
	if err != nil {
		t.Errorf("Error creating workspace from workspace: %v", err)
	}

	content, err := g.ReadFileInWorkspace(context.Background(), "file.txt", ReadFileInWorkspaceOptions{
		WorkspaceID: newID,
	})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if !bytes.Equal(content, []byte("hello world")) {
		t.Errorf("Unexpected content: %s", content)
	}

	err = g.DeleteWorkspace(context.Background(), id)
	if err != nil {
		t.Errorf("Error deleting workspace: %v", err)
	}

	err = g.DeleteWorkspace(context.Background(), newID)
	if err != nil {
		t.Errorf("Error deleting new workspace: %v", err)
	}
}

func TestCreateAndDeleteS3WorkspaceFromWorkspaceDirectory(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" || os.Getenv("WORKSPACE_PROVIDER_S3_BUCKET") == "" {
		t.Skip("Skipping test because AWS credentials are not set")
	}

	id, err := g.CreateWorkspace(context.Background(), "s3")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err = g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "file.txt", []byte("hello world"), WriteFileInWorkspaceOptions{
		WorkspaceID: id,
	})
	if err != nil {
		t.Errorf("Error creating file: %v", err)
	}

	newID, err := g.CreateWorkspace(context.Background(), "directory", id)
	if err != nil {
		t.Errorf("Error creating workspace from workspace: %v", err)
	}

	content, err := g.ReadFileInWorkspace(context.Background(), "file.txt", ReadFileInWorkspaceOptions{
		WorkspaceID: newID,
	})
	if err != nil {
		t.Fatalf("Error reading file: %v", err)
	}

	if !bytes.Equal(content, []byte("hello world")) {
		t.Errorf("Unexpected content: %s", content)
	}

	err = g.DeleteWorkspace(context.Background(), id)
	if err != nil {
		t.Errorf("Error deleting workspace: %v", err)
	}
}

func TestWriteReadAndDeleteFileFromWorkspaceS3(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" || os.Getenv("WORKSPACE_PROVIDER_S3_BUCKET") == "" {
		t.Skip("Skipping test because AWS credentials are not set")
	}

	id, err := g.CreateWorkspace(context.Background(), "s3")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	content, err := g.ReadFileInWorkspace(context.Background(), "test.txt", ReadFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if !bytes.Equal(content, []byte("test")) {
		t.Errorf("Unexpected content: %s", content)
	}

	// Read the file and request the revision ID
	contentWithRevision, err := g.ReadFileWithRevisionInWorkspace(context.Background(), "test.txt", ReadFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if !bytes.Equal(contentWithRevision.Content, []byte("test")) {
		t.Errorf("Unexpected content: %s", contentWithRevision.Content)
	}

	if contentWithRevision.RevisionID == "" {
		t.Errorf("Expected file revision ID when requesting it: %s", contentWithRevision.RevisionID)
	}

	// Stat the file to ensure it exists
	fileInfo, err := g.StatFileInWorkspace(context.Background(), "test.txt", StatFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error statting file: %v", err)
	}

	if fileInfo.WorkspaceID != id {
		t.Errorf("Unexpected file workspace ID: %v", fileInfo.WorkspaceID)
	}

	if fileInfo.Name != "test.txt" {
		t.Errorf("Unexpected file name: %s", fileInfo.Name)
	}

	if fileInfo.Size != 4 {
		t.Errorf("Unexpected file size: %d", fileInfo.Size)
	}

	if fileInfo.ModTime.IsZero() {
		t.Errorf("Unexpected file mod time: %v", fileInfo.ModTime)
	}

	if fileInfo.MimeType != "text/plain" {
		t.Errorf("Unexpected file mime type: %s", fileInfo.MimeType)
	}

	if fileInfo.RevisionID != "" {
		t.Errorf("Unexpected file revision ID when not requesting it: %s", fileInfo.RevisionID)
	}

	// Stat file and request the revision ID
	fileInfo, err = g.StatFileInWorkspace(context.Background(), "test.txt", StatFileInWorkspaceOptions{WorkspaceID: id, WithLatestRevisionID: true})
	if err != nil {
		t.Errorf("Error statting file: %v", err)
	}

	if fileInfo.WorkspaceID != id {
		t.Errorf("Unexpected file workspace ID: %v", fileInfo.WorkspaceID)
	}

	if fileInfo.RevisionID == "" {
		t.Errorf("Expected file revision ID when requesting it: %s", fileInfo.RevisionID)
	}

	// Ensure we get the error we expect when trying to read a non-existent file
	_, err = g.ReadFileInWorkspace(context.Background(), "test1.txt", ReadFileInWorkspaceOptions{WorkspaceID: id})
	if nf := (*NotFoundInWorkspaceError)(nil); !errors.As(err, &nf) {
		t.Errorf("Unexpected error reading non-existent file: %v", err)
	}

	err = g.DeleteFileInWorkspace(context.Background(), "test.txt", DeleteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting file: %v", err)
	}
}

func TestRevisionsForFileInWorkspaceS3(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" || os.Getenv("WORKSPACE_PROVIDER_S3_BUCKET") == "" {
		t.Skip("Skipping test because AWS credentials are not set")
	}

	id, err := g.CreateWorkspace(context.Background(), "s3")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test0"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test1"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test2"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	revisions, err := g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 2 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	for i, rev := range revisions {
		if rev.WorkspaceID != id {
			t.Errorf("Unexpected file workspace ID: %v", rev.WorkspaceID)
		}

		if rev.Name != "test.txt" {
			t.Errorf("Unexpected file name: %s", rev.Name)
		}

		if rev.Size != 5 {
			t.Errorf("Unexpected file size: %d", rev.Size)
		}

		if rev.ModTime.IsZero() {
			t.Errorf("Unexpected file mod time: %v", rev.ModTime)
		}

		if rev.MimeType != "text/plain" {
			t.Errorf("Unexpected file mime type: %s", rev.MimeType)
		}

		if rev.RevisionID != fmt.Sprintf("%d", i+1) {
			t.Errorf("Unexpected revision ID: %s", rev.RevisionID)
		}
	}

	err = g.DeleteRevisionForFileInWorkspace(context.Background(), "test.txt", "1", DeleteRevisionForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting revision for file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 1 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	err = g.DeleteFileInWorkspace(context.Background(), "test.txt", DeleteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 0 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}
}

func TestConflictsForFileInWorkspaceS3(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" || os.Getenv("WORKSPACE_PROVIDER_S3_BUCKET") == "" {
		t.Skip("Skipping test because AWS credentials are not set")
	}

	id, err := g.CreateWorkspace(context.Background(), "s3")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	ce := (*ConflictInWorkspaceError)(nil)
	// Writing a new file with a non-zero latest revision should fail
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test0"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: "1"})
	if err == nil || !errors.As(err, &ce) {
		t.Errorf("Expected error writing file with non-zero latest revision: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test0"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test1"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	revisions, err := g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 1 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	// Writing to the file with the latest revision should succeed
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test2"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: revisions[0].RevisionID})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 2 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	// Writing to the file with the same revision should fail
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test3"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: revisions[0].RevisionID})
	if err == nil || !errors.As(err, &ce) {
		t.Errorf("Expected error writing file with same revision: %v", err)
	}

	latestRevisionID := revisions[1].RevisionID
	err = g.DeleteRevisionForFileInWorkspace(context.Background(), "test.txt", latestRevisionID, DeleteRevisionForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting revision for file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 1 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	// Ensure we cannot write a new file with the zero-th revision ID
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test4"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: revisions[0].RevisionID})
	if err == nil || !errors.As(err, &ce) {
		t.Fatalf("Error creating file: %v", err)
	}

	// Ensure we can write a new file after deleting the latest revision
	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test4"), WriteFileInWorkspaceOptions{WorkspaceID: id, LatestRevisionID: latestRevisionID})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.DeleteFileInWorkspace(context.Background(), "test.txt", DeleteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting file: %v", err)
	}
}

func TestDisableCreatingRevisionsForFileInWorkspaceS3(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" || os.Getenv("WORKSPACE_PROVIDER_S3_BUCKET") == "" {
		t.Skip("Skipping test because AWS credentials are not set")
	}

	id, err := g.CreateWorkspace(context.Background(), "s3")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test0"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test1"), WriteFileInWorkspaceOptions{WorkspaceID: id, CreateRevision: new(bool)})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test.txt", []byte("test2"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	revisions, err := g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 1 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	for i, rev := range revisions {
		if rev.WorkspaceID != id {
			t.Errorf("Unexpected file workspace ID: %v", rev.WorkspaceID)
		}

		if rev.Name != "test.txt" {
			t.Errorf("Unexpected file name: %s", rev.Name)
		}

		if rev.Size != 5 {
			t.Errorf("Unexpected file size: %d", rev.Size)
		}

		if rev.ModTime.IsZero() {
			t.Errorf("Unexpected file mod time: %v", rev.ModTime)
		}

		if rev.MimeType != "text/plain" {
			t.Errorf("Unexpected file mime type: %s", rev.MimeType)
		}

		if rev.RevisionID != fmt.Sprintf("%d", i+1) {
			t.Errorf("Unexpected revision ID: %s", rev.RevisionID)
		}
	}

	err = g.DeleteRevisionForFileInWorkspace(context.Background(), "test.txt", "1", DeleteRevisionForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting revision for file: %v", err)
	}

	revisions, err = g.ListRevisionsForFileInWorkspace(context.Background(), "test.txt", ListRevisionsForFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if len(revisions) != 0 {
		t.Errorf("Unexpected number of revisions: %d", len(revisions))
	}

	err = g.DeleteFileInWorkspace(context.Background(), "test.txt", DeleteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Errorf("Error deleting file: %v", err)
	}
}

func TestLsComplexWorkspaceS3(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" || os.Getenv("WORKSPACE_PROVIDER_S3_BUCKET") == "" {
		t.Skip("Skipping test because AWS credentials are not set")
	}

	id, err := g.CreateWorkspace(context.Background(), "s3")
	if err != nil {
		t.Fatalf("Error creating workspace: %v", err)
	}

	t.Cleanup(func() {
		err := g.DeleteWorkspace(context.Background(), id)
		if err != nil {
			t.Errorf("Error deleting workspace: %v", err)
		}
	})

	err = g.WriteFileInWorkspace(context.Background(), "test/test1.txt", []byte("hello1"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test1/test2.txt", []byte("hello2"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), "test1/test3.txt", []byte("hello3"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}

	err = g.WriteFileInWorkspace(context.Background(), ".hidden.txt", []byte("hidden"), WriteFileInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error creating hidden file: %v", err)
	}

	// List all files
	content, err := g.ListFilesInWorkspace(context.Background(), ListFilesInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error listing files: %v", err)
	}

	if len(content) != 4 {
		t.Errorf("Unexpected number of files: %d", len(content))
	}

	// List files in subdirectory
	content, err = g.ListFilesInWorkspace(context.Background(), ListFilesInWorkspaceOptions{WorkspaceID: id, Prefix: "test1"})
	if err != nil {
		t.Fatalf("Error listing files: %v", err)
	}

	if len(content) != 2 {
		t.Errorf("Unexpected number of files: %d", len(content))
	}

	// Remove all files with test1 prefix
	err = g.RemoveAll(context.Background(), RemoveAllOptions{WorkspaceID: id, WithPrefix: "test1"})
	if err != nil {
		t.Fatalf("Error removing files: %v", err)
	}

	// List files in subdirectory
	content, err = g.ListFilesInWorkspace(context.Background(), ListFilesInWorkspaceOptions{WorkspaceID: id})
	if err != nil {
		t.Fatalf("Error listing files: %v", err)
	}

	if len(content) != 2 {
		t.Errorf("Unexpected number of files: %d", len(content))
	}
}
