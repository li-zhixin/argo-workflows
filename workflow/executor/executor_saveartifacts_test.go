package executor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
)

func TestSaveArtifactsFlow(t *testing.T) {
	// Setup test directory
	tempDir := t.TempDir()
	artifactDir := filepath.Join(tempDir, "report")
	require.NoError(t, os.MkdirAll(artifactDir, 0755))

	// Create test files
	indexFile := filepath.Join(artifactDir, "index.html")
	require.NoError(t, os.WriteFile(indexFile, []byte("<html><body>Test Report</body></html>"), 0644))

	// Create a minimal template with previewPath
	template := wfv1.Template{
		Name: "test-template",
		Outputs: wfv1.Outputs{
			Artifacts: wfv1.Artifacts{
				{
					Name:        "test-report",
					Path:        artifactDir,
					PreviewPath: "index.html",
					Archive: &wfv1.ArchiveStrategy{
						None: &wfv1.NoneStrategy{},
					},
				},
			},
		},
		// Minimal archive location for testing
		ArchiveLocation: &wfv1.ArtifactLocation{
			S3: &wfv1.S3Artifact{
				S3Bucket: wfv1.S3Bucket{
					Bucket: "test-bucket",
				},
				Key: "test-workflow",
			},
		},
	}

	// Test 1: Verify the template artifact has previewPath
	templateArtifact := template.Outputs.Artifacts[0]
	assert.Equal(t, "index.html", templateArtifact.PreviewPath, "Template artifact should have previewPath")

	// Test 2: Simulate the SaveArtifacts loop logic without actually saving
	var savedArtifacts wfv1.Artifacts

	for _, art := range template.Outputs.Artifacts {
		t.Logf("Processing artifact: %s, previewPath: %s", art.Name, art.PreviewPath)

		// Simulate what happens in saveArtifact - just basic validation
		if art.Path == "" {
			t.Errorf("Artifact path is empty")
			continue
		}

		// Check if the artifact path exists
		if _, err := os.Stat(art.Path); err != nil {
			t.Logf("Artifact path doesn't exist (expected in some cases): %v", err)
			continue
		}

		// This simulates the key part: the artifact is added to the result
		// In the real code, this is where the artifact might lose fields
		savedArtifacts = append(savedArtifacts, art)
	}

	// Test 3: Verify previewPath is preserved in saved artifacts
	require.Len(t, savedArtifacts, 1, "Should have one saved artifact")
	savedArtifact := savedArtifacts[0]

	assert.Equal(t, "test-report", savedArtifact.Name, "Name should be preserved")
	assert.Equal(t, artifactDir, savedArtifact.Path, "Path should be preserved")
	assert.Equal(t, "index.html", savedArtifact.PreviewPath, "PreviewPath should be preserved")

	// Test 4: Test the ReportOutputs logic
	outputs := template.Outputs.DeepCopy()
	outputs.Artifacts = savedArtifacts

	require.Len(t, outputs.Artifacts, 1)
	reportedArtifact := outputs.Artifacts[0]
	assert.Equal(t, "index.html", reportedArtifact.PreviewPath, "PreviewPath should be preserved in ReportOutputs")

	// Test 5: JSON serialization (simulating API response)
	jsonData, err := json.Marshal(reportedArtifact)
	require.NoError(t, err)

	var deserializedArtifact wfv1.Artifact
	require.NoError(t, json.Unmarshal(jsonData, &deserializedArtifact))
	assert.Equal(t, "index.html", deserializedArtifact.PreviewPath, "PreviewPath should survive JSON serialization")

	t.Logf("Final artifact JSON: %s", string(jsonData))
}

func TestArtifactModificationDuringProcessing(t *testing.T) {
	// This test simulates potential issues during artifact processing

	original := wfv1.Artifact{
		Name:        "test-artifact",
		Path:        "/test/path",
		PreviewPath: "index.html",
		Archive: &wfv1.ArchiveStrategy{
			None: &wfv1.NoneStrategy{},
		},
	}

	t.Logf("Original artifact: %+v", original)

	// Test what happens when we modify storage-related fields
	// This simulates what happens in saveArtifactFromFile
	artifact := original.DeepCopy()

	// Simulate setting storage location (like what happens in saveArtifactFromFile)
	artifact.S3 = &wfv1.S3Artifact{
		S3Bucket: wfv1.S3Bucket{
			Bucket: "test-bucket",
		},
		Key: "test-key/artifact.tgz",
	}

	t.Logf("After setting S3 location: %+v", *artifact)

	// Verify that previewPath is still preserved
	assert.Equal(t, original.PreviewPath, artifact.PreviewPath, "PreviewPath should be preserved after setting storage location")
	assert.Equal(t, original.Name, artifact.Name, "Name should be preserved")
	assert.Equal(t, original.Path, artifact.Path, "Path should be preserved")

	// Test JSON serialization
	jsonData, err := json.Marshal(*artifact)
	require.NoError(t, err)

	var unmarshaled wfv1.Artifact
	require.NoError(t, json.Unmarshal(jsonData, &unmarshaled))

	assert.Equal(t, original.PreviewPath, unmarshaled.PreviewPath, "PreviewPath should survive JSON roundtrip")

	t.Logf("JSON representation: %s", string(jsonData))
}
