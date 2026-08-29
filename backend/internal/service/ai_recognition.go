package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"oscraper/internal/media"
	"oscraper/internal/provider/ai"
)

type AIRecognitionProvider interface {
	Recognize(ctx context.Context, config ai.Config, fileName, relativePath, libraryType string) (*ai.Result, error)
}

type AIRecognitionService struct {
	settings *SettingService
	provider AIRecognitionProvider
	mu       sync.Mutex
	requests []time.Time
}

func NewAIRecognitionService(settings *SettingService, provider AIRecognitionProvider) *AIRecognitionService {
	return &AIRecognitionService{settings: settings, provider: provider}
}

func (s *AIRecognitionService) Recognize(ctx context.Context, fileName, relativePath, libraryType string) (media.Info, bool, error) {
	config, hasKey, err := s.settings.AIConfig()
	if err != nil {
		return media.Info{}, false, err
	}
	if !config.Enabled || !hasKey || s.provider == nil {
		return media.Info{}, false, nil
	}
	if err := s.wait(ctx, config.QPMLimit); err != nil {
		return media.Info{}, false, err
	}
	result, err := s.provider.Recognize(ctx, config, fileName, relativePath, libraryType)
	if err != nil {
		return media.Info{}, false, mapAIError(err)
	}
	if result == nil || !result.Success || strings.TrimSpace(result.Title) == "" {
		return media.Info{}, false, nil
	}
	return media.Info{
		Title: strings.TrimSpace(result.Title), Year: result.Year, Season: result.Season,
		Episode: result.Episode, Confidence: 95,
	}, true, nil
}

func (s *AIRecognitionService) wait(ctx context.Context, qpm int) error {
	if qpm < 1 {
		qpm = 60
	}
	for {
		now := time.Now()
		cutoff := now.Add(-time.Minute)
		s.mu.Lock()
		firstCurrent := 0
		for firstCurrent < len(s.requests) && s.requests[firstCurrent].Before(cutoff) {
			firstCurrent++
		}
		s.requests = append([]time.Time(nil), s.requests[firstCurrent:]...)
		if len(s.requests) < qpm {
			s.requests = append(s.requests, now)
			s.mu.Unlock()
			return nil
		}
		waitFor := time.Until(s.requests[0].Add(time.Minute))
		s.mu.Unlock()
		if waitFor < time.Millisecond {
			continue
		}
		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
