package repository

import (
	"fmt"
	"testing"

	"oscraper/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestActiveJobsAreScopedSeparatelyByTargetAndCandidate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ScrapeJob{}); err != nil {
		t.Fatal(err)
	}
	jobs := []model.ScrapeJob{
		{TargetID: 1, CandidateID: 11, Status: "pending", Stage: "preparing"},
		{TargetID: 2, CandidateID: 22, Status: "running", Stage: "renaming"},
		{TargetID: 1, CandidateID: 33, Status: "succeeded", Stage: "completed"},
	}
	if err := db.Create(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewJobRepository(db)

	targetActive, err := repository.ActiveByTarget(1)
	if err != nil {
		t.Fatal(err)
	}
	candidateActive, err := repository.ActiveByCandidate(33)
	if err != nil {
		t.Fatal(err)
	}
	if targetActive != 1 || candidateActive != 0 {
		t.Fatalf("target activity leaked into candidate activity: target=%d candidate=%d", targetActive, candidateActive)
	}

	otherTargetActive, err := repository.ActiveByTarget(2)
	if err != nil {
		t.Fatal(err)
	}
	otherCandidateActive, err := repository.ActiveByCandidate(22)
	if err != nil {
		t.Fatal(err)
	}
	if otherTargetActive != 1 || otherCandidateActive != 1 {
		t.Fatalf("running job was not counted in both explicit scopes: target=%d candidate=%d", otherTargetActive, otherCandidateActive)
	}
}
