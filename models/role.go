package models

type Role struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"unique"`
}

func (Role) TableName() string {
	return "auth.roles"
}
