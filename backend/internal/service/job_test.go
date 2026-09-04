package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"oscraper/config"
	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/pkg/cryptoutil"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type memoryOpenList struct {
	mu             sync.Mutex
	entries        map[string]bool
	uploads        map[string]string
	failUploadOnce bool
	uploadCalls    int
}

func (m *memoryOpenList) ListDirectory(_ context.Context, _, _, directory string, _ bool) ([]openlist.DirectoryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]openlist.DirectoryEntry, 0)
	for remotePath, isDir := range m.entries {
		if path.Dir(remotePath) == directory {
			result = append(result, openlist.DirectoryEntry{Name: path.Base(remotePath), Path: remotePath, IsDir: isDir, Size: 1, Modified: "v1"})
		}
	}
	return result, nil
}

func (m *memoryOpenList) CreateDirectory(_ context.Context, _, _, remotePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.entries[remotePath]; exists {
		return errors.New("already exists")
	}
	m.entries[remotePath] = true
	return nil
}

func (m *memoryOpenList) RenameEntry(_ context.Context, _, _, sourcePath, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	isDir, exists := m.entries[sourcePath]
	if !exists {
		return errors.New("source missing")
	}
	target := path.Join(path.Dir(sourcePath), newName)
	if _, exists := m.entries[target]; exists {
		return errors.New("target exists")
	}
	delete(m.entries, sourcePath)
	m.entries[target] = isDir
	if isDir {
		m.rewriteChildren(sourcePath, target)
	}
	return nil
}

func (m *memoryOpenList) MoveEntries(_ context.Context, _, _, sourceDirectory, targetDirectory string, names []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, name := range names {
		source, target := path.Join(sourceDirectory, name), path.Join(targetDirectory, name)
		isDir, exists := m.entries[source]
		if !exists {
			return errors.New("source missing")
		}
		if _, exists := m.entries[target]; exists {
			return errors.New("target exists")
		}
		delete(m.entries, source)
		m.entries[target] = isDir
		if isDir {
			m.rewriteChildren(source, target)
		}
	}
	return nil
}

func (m *memoryOpenList) Upload(_ context.Context, _, _, remotePath, _ string, _ int64, content io.Reader) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploadCalls++
	if m.failUploadOnce {
		m.failUploadOnce = false
		return errors.New("injected upload failure")
	}
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	if m.uploads == nil {
		m.uploads = make(map[string]string)
	}
	m.uploads[remotePath] = string(data)
	m.entries[remotePath] = false
	return nil
}

func (m *memoryOpenList) rewriteChildren(source, target string) {
	for existing, isDir := range m.entries {
		if strings.HasPrefix(existing, source+"/") {
			delete(m.entries, existing)
			m.entries[target+strings.TrimPrefix(existing, source)] = isDir
		}
	}
}

func newJobTestService(t *testing.T, remote *memoryOpenList) (*JobService, *gorm.DB, *model.ScrapePreview) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:job-%s-%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// Job execution dispatches worker goroutines while tests poll the result; the
	// shared in-memory database needs a single connection to avoid table lock errors.
	if sqlDB, err := db.DB(); err != nil {
		t.Fatal(err)
	} else {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.OpenListConnection{}, &model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{}, &model.ScrapeJob{}, &model.ScrapeJobOperation{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	cipher, _ := cryptoutil.New("0123456789abcdef0123456789abcdef")
	encrypted, _ := cipher.Encrypt("token")
	connection := model.OpenListConnection{Name: "Home", BaseURL: "http://openlist.example", BasePath: "/movies", EncryptedToken: encrypted, Enabled: true}
	if err := db.Create(&connection).Error; err != nil {
		t.Fatal(err)
	}
	connectionID := connection.ID
	target := model.ScrapeTarget{SourceType: "openlist", ConnectionID: &connectionID, Name: "Movies", RootPath: "/movies", LibraryType: "movie", RenameEnabled: true, Enabled: true}
	_ = db.Create(&target).Error
	startedAt := time.Now()
	scan := model.ScanRun{TargetID: target.ID, Status: "succeeded", StartedAt: &startedAt}
	_ = db.Create(&scan).Error
	candidate := model.MediaCandidate{ScanID: scan.ID, TargetID: target.ID, Path: "/movies/Arrival.mkv", Kind: "movie", Fingerprint: "sha256:fresh", RepresentativeFile: "Arrival.mkv", Status: "ready", VideoCount: 1}
	_ = db.Create(&candidate).Error
	finalRoot := "/movies/Arrival (2016) {tmdbid-329865}"
	plan := PreviewPlan{
		ReadOnly: true, Ready: true, RenameAllowed: true, OrganizeFlatMovie: true, SourcePath: candidate.Path,
		ProposedDirectoryName: path.Base(finalRoot), ProposedDirectoryPath: finalRoot,
		ProposedDirectoryCreates: []string{finalRoot}, ProposedDirectoryRenames: []RenameItem{},
		ProposedFileRenames: []RenameItem{{SourcePath: candidate.Path, TargetPath: finalRoot + "/Arrival (2016) {tmdbid-329865}.mkv", AssetType: "video"}},
		Artifacts:           []PreviewArtifact{{Path: finalRoot + "/Arrival (2016) {tmdbid-329865}.nfo", Kind: "nfo", Content: "<movie><title>Arrival</title></movie>"}},
		GeneratedFiles:      []string{finalRoot + "/Arrival (2016) {tmdbid-329865}.nfo"}, Warnings: []string{}, Conflicts: []PlanConflict{},
	}
	planJSON, _ := json.Marshal(plan)
	preview := model.ScrapePreview{TargetID: target.ID, CandidateID: candidate.ID, ActorID: 1, TMDBID: 329865, MediaType: "movie", Fingerprint: candidate.Fingerprint, MatchJSON: `{}`, PlanJSON: string(planJSON), ExpiresAt: time.Now().Add(time.Hour)}
	_ = db.Create(&preview).Error
	inspector := stubCandidateInspector{inspection: &CandidateInspection{Candidate: &candidate, Entries: []openlist.DirectoryEntry{{Name: "Arrival.mkv", Path: candidate.Path, Size: 1, Modified: "v1"}}, Fingerprint: candidate.Fingerprint}}
	cfg := &config.Config{SQLitePath: t.TempDir() + "/db.sqlite", JobWorkDir: t.TempDir(), ScrapeWorkers: 1, ScrapeQueueSize: 2, MaxImageBytes: 2 << 20, HTTPTimeoutSeconds: 2}
	service, err := NewJobService(db, cfg, cipher, remote, inspector, NewConnectionQuota())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Shutdown(context.Background()) })
	return service, db, &preview
}

func waitJob(t *testing.T, service *JobService, id uint) *model.ScrapeJob {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status == "succeeded" || job.Status == "failed" || job.Status == "canceled" {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not reach a terminal state")
	return nil
}

func TestJobExecutesAndVerifiesFlatMoviePlan(t *testing.T) {
	remote := &memoryOpenList{entries: map[string]bool{"/movies/Arrival.mkv": false}}
	service, _, preview := newJobTestService(t, remote)
	job, err := service.Submit(1, 1, SubmitJobCommand{PreviewID: preview.ID, RenameMedia: true, ConfirmDirectoryFingerprint: preview.Fingerprint}, "same-request")
	if err != nil {
		t.Fatal(err)
	}
	job = waitJob(t, service, job.ID)
	if job.Status != "succeeded" || job.Progress != 100 {
		t.Fatalf("unexpected job: %#v", job)
	}
	remote.mu.Lock()
	_, videoExists := remote.entries["/movies/Arrival (2016) {tmdbid-329865}/Arrival (2016) {tmdbid-329865}.mkv"]
	_, nfoExists := remote.entries["/movies/Arrival (2016) {tmdbid-329865}/Arrival (2016) {tmdbid-329865}.nfo"]
	markerContent := remote.uploads["/movies/Arrival (2016) {tmdbid-329865}/"+scrapeMarkerName]
	remote.mu.Unlock()
	if !videoExists || !nfoExists || markerContent != scrapeMarkerContent {
		t.Fatalf("final OpenList state is incomplete: %#v", remote.entries)
	}
	same, err := service.Submit(1, 1, SubmitJobCommand{PreviewID: preview.ID, RenameMedia: true, ConfirmDirectoryFingerprint: preview.Fingerprint}, "same-request")
	if err != nil || same.ID != job.ID {
		t.Fatalf("idempotency key did not return the original job: %#v %v", same, err)
	}
}

func TestFailedUploadRetriesFromOperationCheckpoint(t *testing.T) {
	remote := &memoryOpenList{entries: map[string]bool{"/movies/Arrival.mkv": false}, failUploadOnce: true}
	service, _, preview := newJobTestService(t, remote)
	job, err := service.Submit(1, 1, SubmitJobCommand{PreviewID: preview.ID, RenameMedia: true, ConfirmDirectoryFingerprint: preview.Fingerprint}, "")
	if err != nil {
		t.Fatal(err)
	}
	job = waitJob(t, service, job.ID)
	if job.Status != "failed" {
		t.Fatalf("injected failure did not fail job: %#v", job)
	}
	retried, err := service.Retry(job.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	retried = waitJob(t, service, retried.ID)
	if retried.Status != "succeeded" || retried.Attempts != 2 || remote.uploadCalls != 3 {
		t.Fatalf("retry did not resume safely: %#v uploads=%d", retried, remote.uploadCalls)
	}
}

func TestExistingMetadataSkipsUpload(t *testing.T) {
	finalRoot := "/movies/Arrival (2016) {tmdbid-329865}"
	finalNFO := finalRoot + "/Arrival (2016) {tmdbid-329865}.nfo"
	remote := &memoryOpenList{entries: map[string]bool{
		"/movies/Arrival.mkv": false,
		finalRoot:             true,
		finalNFO:              false,
	}}
	service, _, preview := newJobTestService(t, remote)
	job, err := service.Submit(1, 1, SubmitJobCommand{PreviewID: preview.ID, RenameMedia: true, ConfirmDirectoryFingerprint: preview.Fingerprint}, "")
	if err != nil {
		t.Fatal(err)
	}
	job = waitJob(t, service, job.ID)
	if job.Status != "succeeded" || remote.uploadCalls != 1 {
		t.Fatalf("existing metadata was uploaded: job=%#v uploads=%d", job, remote.uploadCalls)
	}
	operations, err := service.Operations(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	metadataOperation := operations[len(operations)-2]
	markerOperation := operations[len(operations)-1]
	if metadataOperation.Type != "upload" || metadataOperation.Status != "skipped" || metadataOperation.LastError != "" || markerOperation.Type != "marker" || markerOperation.Status != "succeeded" {
		t.Fatalf("existing metadata operation was not cleanly skipped: metadata=%#v marker=%#v", metadataOperation, markerOperation)
	}
}

func TestLocalJobRenamesMediaAndWritesMetadata(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "movies")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	sourceVideo := filepath.Join(library, "Arrival.mkv")
	if err := os.WriteFile(sourceVideo, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:local-job-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.OpenListConnection{}, &model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{}, &model.ScrapeJob{}, &model.ScrapeJobOperation{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	target := model.ScrapeTarget{SourceType: "local", Name: "Local movies", RootPath: filepath.ToSlash(library), LibraryType: "movie", RenameEnabled: true, Enabled: true}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now()
	scan := model.ScanRun{TargetID: target.ID, Status: "succeeded", StartedAt: &startedAt}
	_ = db.Create(&scan).Error
	candidate := model.MediaCandidate{ScanID: scan.ID, TargetID: target.ID, Path: filepath.ToSlash(sourceVideo), Kind: "movie", Fingerprint: "sha256:local", RepresentativeFile: "Arrival.mkv", Status: "ready", VideoCount: 1}
	_ = db.Create(&candidate).Error
	finalRoot := filepath.ToSlash(filepath.Join(library, "Arrival (2016) {tmdbid-329865}"))
	finalVideo := finalRoot + "/Arrival (2016) {tmdbid-329865}.mkv"
	finalNFO := finalRoot + "/Arrival (2016) {tmdbid-329865}.nfo"
	plan := PreviewPlan{
		ReadOnly: true, Ready: true, RenameAllowed: true, OrganizeFlatMovie: true, SourcePath: candidate.Path,
		ProposedDirectoryName: path.Base(finalRoot), ProposedDirectoryPath: finalRoot,
		ProposedDirectoryCreates: []string{finalRoot}, ProposedDirectoryRenames: []RenameItem{},
		ProposedFileRenames: []RenameItem{{SourcePath: candidate.Path, TargetPath: finalVideo, AssetType: "video"}},
		Artifacts:           []PreviewArtifact{{Path: finalNFO, Kind: "nfo", Content: "<movie><title>Arrival</title></movie>"}},
		GeneratedFiles:      []string{finalNFO}, Warnings: []string{}, Conflicts: []PlanConflict{},
	}
	planJSON, _ := json.Marshal(plan)
	preview := model.ScrapePreview{TargetID: target.ID, CandidateID: candidate.ID, ActorID: 1, TMDBID: 329865, MediaType: "movie", Fingerprint: candidate.Fingerprint, MatchJSON: `{}`, PlanJSON: string(planJSON), ExpiresAt: time.Now().Add(time.Hour)}
	_ = db.Create(&preview).Error
	inspector := stubCandidateInspector{inspection: &CandidateInspection{Candidate: &candidate, Fingerprint: candidate.Fingerprint}}
	cipher, _ := cryptoutil.New("0123456789abcdef0123456789abcdef")
	cfg := &config.Config{SQLitePath: filepath.Join(root, "db.sqlite"), LocalMediaRoot: root, JobWorkDir: filepath.Join(root, "jobs"), ScrapeWorkers: 1, ScrapeQueueSize: 2, MaxImageBytes: 2 << 20}
	service, err := NewJobService(db, cfg, cipher, &memoryOpenList{}, inspector, NewConnectionQuota())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Shutdown(context.Background()) })
	job, err := service.Submit(target.ID, 1, SubmitJobCommand{PreviewID: preview.ID, RenameMedia: true, ConfirmDirectoryFingerprint: preview.Fingerprint}, "local-job")
	if err != nil {
		t.Fatal(err)
	}
	job = waitJob(t, service, job.ID)
	if job.Status != "succeeded" {
		t.Fatalf("local job failed: %#v", job)
	}
	if _, err := os.Stat(filepath.FromSlash(finalVideo)); err != nil {
		t.Fatalf("renamed local video is missing: %v", err)
	}
	data, err := os.ReadFile(filepath.FromSlash(finalNFO))
	if err != nil || !strings.Contains(string(data), "Arrival") {
		t.Fatalf("local NFO is missing: %q %v", data, err)
	}
	markerData, err := os.ReadFile(filepath.Join(filepath.FromSlash(finalRoot), scrapeMarkerName))
	if err != nil || string(markerData) != scrapeMarkerContent {
		t.Fatalf("local scrape marker is missing or changed: %q %v", markerData, err)
	}
}

func TestDownloadImageStreamsOnlySupportedBoundedContent(t *testing.T) {
	retryRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/valid":
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write([]byte("png!"))
		case "/invalid":
			writer.Header().Set("Content-Type", "text/html")
			_, _ = writer.Write([]byte("no"))
		case "/large":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("too-large"))
		case "/retry":
			retryRequests++
			if retryRequests < imageDownloadAttempts {
				http.Error(writer, "temporary", http.StatusServiceUnavailable)
				return
			}
			writer.Header().Set("Content-Type", "image/webp")
			_, _ = writer.Write([]byte("webp"))
		}
	}))
	defer server.Close()

	executor := &JobExecutor{maxImage: 4, imageClient: server.Client()}
	destination := filepath.Join(t.TempDir(), "poster.png")
	contentType, err := executor.downloadImage(context.Background(), server.URL+"/valid", destination)
	if err != nil || contentType != "image/png" {
		t.Fatalf("valid image failed: type=%q err=%v", contentType, err)
	}
	if content, err := os.ReadFile(destination); err != nil || string(content) != "png!" {
		t.Fatalf("unexpected downloaded content %q: %v", content, err)
	}
	retryDestination := filepath.Join(t.TempDir(), "retry.webp")
	contentType, err = executor.downloadImage(context.Background(), server.URL+"/retry", retryDestination)
	if err != nil || contentType != "image/webp" || retryRequests != imageDownloadAttempts {
		t.Fatalf("transient image failure was not retried: type=%q requests=%d err=%v", contentType, retryRequests, err)
	}
	for _, test := range []struct{ route, code string }{{"/invalid", "job.invalid_image_type"}, {"/large", "job.image_too_large"}} {
		_, err := executor.downloadImage(context.Background(), server.URL+test.route, destination+strings.ReplaceAll(test.route, "/", "-"))
		var serviceError *Error
		if !errors.As(err, &serviceError) || serviceError.Code != test.code {
			t.Fatalf("%s returned %v, want %s", test.route, err, test.code)
		}
	}
}

func TestImageDownloadFailureIsSkippedAfterRetries(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		http.Error(writer, "temporary", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	remote := &memoryOpenList{entries: map[string]bool{"/movies/Arrival.mkv": false}}
	service, db, preview := newJobTestService(t, remote)
	service.executor.imageClient = server.Client()
	var plan PreviewPlan
	if err := json.Unmarshal([]byte(preview.PlanJSON), &plan); err != nil {
		t.Fatal(err)
	}
	imagePath := "/movies/Arrival (2016) {tmdbid-329865}/Arrival (2016) {tmdbid-329865}-poster.jpg"
	plan.Artifacts = append(plan.Artifacts, PreviewArtifact{Path: imagePath, Kind: "poster", SourceURL: server.URL + "/poster"})
	plan.GeneratedFiles = append(plan.GeneratedFiles, imagePath)
	encoded, _ := json.Marshal(plan)
	preview.PlanJSON = string(encoded)
	if err := db.Save(preview).Error; err != nil {
		t.Fatal(err)
	}

	job, err := service.Submit(1, 1, SubmitJobCommand{PreviewID: preview.ID, RenameMedia: true, ConfirmDirectoryFingerprint: preview.Fingerprint}, "")
	if err != nil {
		t.Fatal(err)
	}
	job = waitJob(t, service, job.ID)
	if job.Status != "succeeded" || requests != imageDownloadAttempts {
		t.Fatalf("image failure did not degrade gracefully: job=%#v requests=%d", job, requests)
	}
	operations, err := service.Operations(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	imageOperation := operations[len(operations)-2]
	if imageOperation.ArtifactKind != "poster" || imageOperation.Status != "skipped" || imageOperation.LastError == "" {
		t.Fatalf("failed image operation was not recorded as skipped: %#v", imageOperation)
	}
	remote.mu.Lock()
	_, imageUploaded := remote.entries[imagePath]
	remote.mu.Unlock()
	if imageUploaded {
		t.Fatal("failed image was unexpectedly uploaded")
	}
}

func TestCleanupExpiredWorkspacesIgnoresNonJobDirectoriesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	oldJob := filepath.Join(root, "12")
	keep := filepath.Join(root, "manual")
	if err := os.Mkdir(oldJob, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(keep, 0o750); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(oldJob, old, old); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "13")
	if err := os.Symlink(keep, link); err != nil {
		t.Fatal(err)
	}
	cleanupExpiredWorkspaces(root, 7)
	if _, err := os.Stat(oldJob); !os.IsNotExist(err) {
		t.Fatalf("expired job workspace was not removed: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("non-job directory was removed: %v", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("job-like symlink was removed: %v", err)
	}
}
