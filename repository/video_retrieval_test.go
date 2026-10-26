package repository

import (
	"github.com/libtnb/sqlite"
	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestVideoRetrievalV1BackupMigration(t *testing.T) {
	before, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "before.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = before.AutoMigrate(&model.VideoTask{}, &model.StorageObject{}); err != nil {
		t.Fatal(err)
	}
	task := model.VideoTask{ID: "old-task", UserID: "owner", Status: "completed", UpstreamTaskID: "original-id", ParameterTranslationSnapshot: `{"original":"configuration"}`, RequestBody: "unchanged"}
	if err = before.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "backup.db")
	if err = before.Exec("VACUUM INTO ?", backup).Error; err != nil {
		t.Fatal(err)
	}
	after, err := gorm.Open(sqlite.Open(backup), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = migrateVideoRetrievalV1(after); err != nil {
			t.Fatal(err)
		}
	}
	var got model.VideoTask
	if err = after.First(&got, "id = ?", task.ID).Error; err != nil || got != task {
		t.Fatal("old task changed", err)
	}
	for _, state := range []string{"fetching", "saving", "ready", "failed"} {
		r := model.VideoRetrieval{TaskID: state, UserID: "owner", Status: state, Attempt: 2, ObjectID: state}
		if err = after.Create(&r).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = migrateVideoRetrievalV1(after); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"fetching", "saving", "ready", "failed"} {
		var r model.VideoRetrieval
		if err = after.First(&r, "task_id = ?", state).Error; err != nil {
			t.Fatal(err)
		}
		want := state
		if state == "fetching" || state == "saving" {
			want = "failed"
		}
		if r.Status != want || r.Attempt != 2 || r.ObjectID != state {
			t.Fatal("restart lost result or silently retried")
		}
	}
	// Original binary's table shape still reads the same task after the additive migration.
	if err = after.First(&got, "id = ?", task.ID).Error; err != nil || got != task {
		t.Fatal("rollback read changed")
	}
	if before.Migrator().HasTable(&model.VideoRetrieval{}) {
		t.Fatal("source backup modified")
	}
}
