package repository

import (
	"time"

	"oscraper/internal/model"

	"gorm.io/gorm"
)

type AuditRepository struct{ db *gorm.DB }

func NewAuditRepository(db *gorm.DB) *AuditRepository { return &AuditRepository{db: db} }

func (r *AuditRepository) Record(actorID uint, action, target, detail string) error {
	return r.db.Create(&model.AdminAuditLog{
		ActorID: actorID, Action: action, Target: target, Detail: detail, OccurredAt: time.Now(),
	}).Error
}
