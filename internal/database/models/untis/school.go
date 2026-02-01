package untis

import "time"

type School struct {
	TenantId    string    `json:"tenantId"`
	SchoolId    int       `json:"schoolId"`
	DisplayName string    `json:"displayName"`
	LoginName   string    `json:"loginName"`
	Server      string    `json:"server"`
	Address     string    `json:"address"`
	LastUpdated time.Time `json:"lastUpdated"`
}
