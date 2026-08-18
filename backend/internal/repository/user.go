package repository

import (
	"openlistscraper/internal/model"

	"gorm.io/gorm"
)

type UserRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) CountAdmins() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Where("is_admin = ?", true).Count(&count).Error
	return count, err
}

func (r *UserRepository) Create(user *model.User) error { return r.db.Create(user).Error }

func (r *UserRepository) Update(user *model.User) error { return r.db.Save(user).Error }

func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) IncrementTokenVersion(id uint) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}
