package service

import (
	"context"
	"sync"
	"time"

	"openlistscraper/internal/model"
)

type ConnectionQuota struct {
	mu      sync.Mutex
	history map[uint][]time.Time
}

func NewConnectionQuota() *ConnectionQuota {
	return &ConnectionQuota{history: make(map[uint][]time.Time)}
}

func (q *ConnectionQuota) Wait(ctx context.Context, connection *model.OpenListConnection) error {
	if connection.QPSLimit <= 0 && connection.QPMLimit <= 0 {
		return nil
	}
	for {
		now := time.Now()
		q.mu.Lock()
		history := q.history[connection.ID]
		minuteCutoff := now.Add(-time.Minute)
		firstCurrent := 0
		for firstCurrent < len(history) && history[firstCurrent].Before(minuteCutoff) {
			firstCurrent++
		}
		history = history[firstCurrent:]
		secondCutoff := now.Add(-time.Second)
		secondCount := 0
		var oldestSecond time.Time
		for _, requestTime := range history {
			if !requestTime.Before(secondCutoff) {
				if secondCount == 0 {
					oldestSecond = requestTime
				}
				secondCount++
			}
		}
		qpsReady := connection.QPSLimit <= 0 || secondCount < connection.QPSLimit
		qpmReady := connection.QPMLimit <= 0 || len(history) < connection.QPMLimit
		if qpsReady && qpmReady {
			q.history[connection.ID] = append(history, now)
			q.mu.Unlock()
			return nil
		}
		waitUntil := now.Add(time.Millisecond)
		if !qpsReady {
			waitUntil = oldestSecond.Add(time.Second)
		}
		if !qpmReady {
			minuteReady := history[0].Add(time.Minute)
			if minuteReady.After(waitUntil) {
				waitUntil = minuteReady
			}
		}
		q.history[connection.ID] = history
		q.mu.Unlock()
		timer := time.NewTimer(time.Until(waitUntil))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
