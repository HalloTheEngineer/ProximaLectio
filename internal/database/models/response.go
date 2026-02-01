package models

import (
	"proximaLectio/internal/database/models/untis"
)

type SchoolSearchResponse struct {
	Result struct {
		Schools []untis.School `json:"schools"`
	} `json:"result"`
}
type AppDataResponse struct {
	User struct {
		Email  string `json:"email"`
		Person struct {
			DisplayName string `json:"displayName"`
			ID          int    `json:"id"`
		} `json:"person"`
	} `json:"user"`
}
