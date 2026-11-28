package domain

import "time"

type ObjectInfo struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Key       string    `json:"-"`
	Name      string    `json:"name"`
	SHA256    string    `json:"sha256"`
	MediaType string    `json:"mediaType"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

type BeaconRelease struct {
	ID              string      `json:"id"`
	Version         string      `json:"version"`
	OS              string      `json:"os"`
	Architecture    string      `json:"architecture"`
	Object          ObjectInfo  `json:"object"`
	SBOM            *ObjectInfo `json:"sbom,omitempty"`
	Checksums       *ObjectInfo `json:"checksums,omitempty"`
	Signature       *ObjectInfo `json:"signature,omitempty"`
	Signed          bool        `json:"signed"`
	SignatureStatus string      `json:"signatureStatus"`
	Development     bool        `json:"development"`
	CreatedAt       time.Time   `json:"createdAt"`
}

type BeaconBuild struct {
	ID           string     `json:"id"`
	ReleaseID    string     `json:"releaseId"`
	Version      string     `json:"version"`
	OS           string     `json:"os"`
	Architecture string     `json:"architecture"`
	SHA256       string     `json:"sha256"`
	CreatedBy    string     `json:"createdBy"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastBundleAt *time.Time `json:"lastBundleAt,omitempty"`
}
