package model

type LoginSession struct {
	ID        string `gorm:"primaryKey"`
	UserID    string `gorm:"index"`
	ExpiresAt int64  `gorm:"index"`
}

type Invitation struct {
	ID            string `json:"id" gorm:"primaryKey"`
	CodeHash      string `json:"-" gorm:"uniqueIndex"`
	EncryptedCode string `json:"-" gorm:"type:text"`
	Code          string `json:"code,omitempty" gorm:"-"`
	CreatedBy     string `json:"createdBy"`
	CreatedAt     string `json:"createdAt"`
	ExpiresAt     int64  `json:"expiresAt"`
	UsedBy        string `json:"usedBy"`
	Revoked       bool   `json:"revoked"`
}
