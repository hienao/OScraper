package logging

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"oscraper/config"
	"oscraper/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type queuedLog struct {
	api         *model.APIRequestLog
	application *model.ApplicationLog
}

type Manager struct {
	DB                 *gorm.DB
	queue              chan queuedLog
	done               chan struct{}
	wg                 sync.WaitGroup
	flushMu            sync.Mutex
	batchSize          int
	apiDropped         atomic.Uint64
	applicationDropped atomic.Uint64
}

type Stats struct {
	Queued             int    `json:"queued"`
	Capacity           int    `json:"capacity"`
	APIDropped         uint64 `json:"api_dropped"`
	ApplicationDropped uint64 `json:"application_dropped"`
}

func NewManager(cfg *config.Config) (*Manager, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.APILogPath), 0o755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(cfg.APILogPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Error)})
	if err != nil {
		return nil, err
	}
	if err := db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		return nil, err
	}
	if err := db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&model.APIRequestLog{}, &model.ApplicationLog{}); err != nil {
		return nil, err
	}
	queueSize := cfg.APILogQueueSize
	if queueSize < 100 {
		queueSize = 100
	}
	batchSize := cfg.APILogBatchSize
	if batchSize < 1 {
		batchSize = 100
	}
	manager := &Manager{DB: db, queue: make(chan queuedLog, queueSize), done: make(chan struct{}), batchSize: batchSize}
	manager.wg.Add(1)
	go manager.writeLoop()
	return manager, nil
}

func (m *Manager) Submit(entry model.APIRequestLog) {
	select {
	case m.queue <- queuedLog{api: &entry}:
	default:
		m.apiDropped.Add(1)
	}
}

func (m *Manager) SubmitApplication(entry model.ApplicationLog) {
	select {
	case m.queue <- queuedLog{application: &entry}:
	default:
		m.applicationDropped.Add(1)
	}
}

func (m *Manager) Dropped() (uint64, uint64) { return m.apiDropped.Load(), m.applicationDropped.Load() }

func (m *Manager) Stats() Stats {
	return Stats{Queued: len(m.queue), Capacity: cap(m.queue), APIDropped: m.apiDropped.Load(), ApplicationDropped: m.applicationDropped.Load()}
}

func (m *Manager) writeLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.flush(m.batchSize)
		case <-m.done:
			m.flush(0)
			return
		}
	}
}

func (m *Manager) flush(max int) {
	m.flushMu.Lock()
	defer m.flushMu.Unlock()
	apiEntries := make([]model.APIRequestLog, 0, m.batchSize)
	applicationEntries := make([]model.ApplicationLog, 0, m.batchSize)
	for count := 0; max == 0 || count < max; count++ {
		select {
		case entry := <-m.queue:
			if entry.api != nil {
				apiEntries = append(apiEntries, *entry.api)
			}
			if entry.application != nil {
				applicationEntries = append(applicationEntries, *entry.application)
			}
		default:
			m.persist(apiEntries, applicationEntries)
			return
		}
	}
	m.persist(apiEntries, applicationEntries)
}

func (m *Manager) Flush() { m.flush(0) }

func (m *Manager) persist(apiEntries []model.APIRequestLog, applicationEntries []model.ApplicationLog) {
	if len(apiEntries) > 0 {
		if err := m.DB.CreateInBatches(&apiEntries, len(apiEntries)).Error; err != nil {
			log.Printf("failed to persist API logs: %v", err)
		}
	}
	if len(applicationEntries) > 0 {
		if err := m.DB.CreateInBatches(&applicationEntries, len(applicationEntries)).Error; err != nil {
			log.Printf("failed to persist application logs: %v", err)
		}
	}
}

func (m *Manager) Close() {
	close(m.done)
	m.wg.Wait()
	if sqlDB, err := m.DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
