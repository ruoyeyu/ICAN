package model

// VideoRetrieval tracks delivery independently of the paid upstream task.
type VideoRetrieval struct {
	TaskID      string `gorm:"primaryKey" json:"-"`
	UserID      string `gorm:"index" json:"-"`
	ObjectID    string `json:"-"`
	Status      string `json:"status"`
	Attempt     uint64 `json:"attempt"`
	ErrorDetail string `gorm:"type:text" json:"errorDetail,omitempty"`
}
