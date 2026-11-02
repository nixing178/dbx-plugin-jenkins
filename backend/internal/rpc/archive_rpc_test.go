package rpc

import "testing"

func TestJobUpdateMapsArtifactArchive(t *testing.T) {
	update := jobUpdate(map[string]any{
		"archive": map[string]any{
			"enabled":           true,
			"files":             "dist/**/*.jar",
			"excludes":          "**/*.tmp",
			"fingerprint":       true,
			"onlyIfSuccessful":  true,
			"allowEmptyArchive": false,
			"defaultExcludes":   false,
			"caseSensitive":     true,
		},
	})

	if !update.SetArchive {
		t.Fatal("SetArchive = false, want true")
	}
	archive := update.Archive
	if !archive.Enabled || archive.Files != "dist/**/*.jar" || archive.Excludes != "**/*.tmp" {
		t.Errorf("archive = %+v, want enabled with pattern and excludes", archive)
	}
	if !archive.Fingerprint || !archive.OnlyIfSuccessful || !archive.CaseSensitive {
		t.Errorf("archive = %+v, want boolean flags mapped", archive)
	}
	if archive.DefaultExcludes {
		t.Error("defaultExcludes = true, want false")
	}
}

// 前端没传 defaultExcludes 时按 Jenkins 默认 true 处理，避免把已有配置改成 false。
func TestJobUpdateArchiveDefaults(t *testing.T) {
	update := jobUpdate(map[string]any{
		"archive": map[string]any{"enabled": true, "files": "out/*.zip"},
	})
	if !update.SetArchive {
		t.Fatal("SetArchive = false, want true")
	}
	if !update.Archive.DefaultExcludes {
		t.Error("defaultExcludes = false, want default true")
	}
}

func TestJobUpdateWithoutArchiveKeepsPublishers(t *testing.T) {
	update := jobUpdate(map[string]any{"description": "hi"})
	if update.SetArchive {
		t.Error("SetArchive = true, want false when archive is absent")
	}
}
