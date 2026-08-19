package repository

import (
	"oscraper/internal/model"

	"gorm.io/gorm"
)

type ConnectionRepository struct{ db *gorm.DB }

func NewConnectionRepository(db *gorm.DB) *ConnectionRepository {
	return &ConnectionRepository{db: db}
}

func (r *ConnectionRepository) List() ([]model.OpenListConnection, error) {
	var connections []model.OpenListConnection
	err := r.db.Order("created_at DESC").Find(&connections).Error
	return connections, err
}

func (r *ConnectionRepository) Find(id uint) (*model.OpenListConnection, error) {
	var connection model.OpenListConnection
	if err := r.db.First(&connection, id).Error; err != nil {
		return nil, err
	}
	return &connection, nil
}

func (r *ConnectionRepository) Create(connection *model.OpenListConnection) error {
	return r.db.Create(connection).Error
}

func (r *ConnectionRepository) Update(connection *model.OpenListConnection) error {
	return r.db.Save(connection).Error
}

func (r *ConnectionRepository) Delete(connection *model.OpenListConnection) error {
	return r.db.Delete(connection).Error
}
