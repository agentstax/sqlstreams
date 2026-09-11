package common

import (
	"uuid"
)

// TODO - must validate json serializable for producer / consumer

type Work struct {
	Id    string `json:"id"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

func (Work) SchemaVersion() int { return 1 }

func NewWork(age int, email string) (*Work, error) {
	id := uuid.NewV7()

	return &Work{
		Id:    id.String(),
		Age:   age,
		Email: email,
	}, nil
}
