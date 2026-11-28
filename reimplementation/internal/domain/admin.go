package domain

import "time"

type ArtifactInfo struct {
	Host         string    `json:"host"`
	Name         string    `json:"name"`
	Length       int64     `json:"length"`
	CreationDate time.Time `json:"creationDate"`
}

type ArtifactContentInfo struct {
	ArtifactInfo
	SHA256    string `json:"sha256"`
	MediaType string `json:"mediaType"`
}

type LicenseInfo struct {
	Email string `json:"email"`
	Key   string `json:"key"`
}

type LicenseValidationInfoEx struct {
	Key             string    `json:"key"`
	Success         bool      `json:"success"`
	License         string    `json:"license"`
	ItemID          string    `json:"item_id"`
	ItemName        string    `json:"item_name"`
	Checksum        string    `json:"checksum"`
	Expires         time.Time `json:"expires"`
	PaymentID       int       `json:"payment_id"`
	CustomerName    string    `json:"customer_name"`
	CustomerEmail   string    `json:"customer_email"`
	LicenseLimit    string    `json:"license_limit"`
	SiteCount       string    `json:"site_count"`
	ActivationsLeft string    `json:"activations_left"`
	PriceID         string    `json:"price_id"`
}

type SerializableVersion struct {
	Major    int `json:"major"`
	Minor    int `json:"minor"`
	Build    int `json:"build"`
	Revision int `json:"revision"`
}

type UpdateJobInfo struct {
	Version           SerializableVersion `json:"version"`
	Status            string              `json:"status"`
	StatusDescription string              `json:"statusDescription"`
	LastUpdateResult  string              `json:"lastUpdateResult"`
	Progress          int                 `json:"progress"`
}

type UpdatePackageInfo struct {
	Version   SerializableVersion `json:"version"`
	SHA256    string              `json:"sha256"`
	MediaType string              `json:"mediaType"`
	Length    int64               `json:"length"`
	Active    bool                `json:"active"`
	CreatedAt time.Time           `json:"createdAt"`
}
