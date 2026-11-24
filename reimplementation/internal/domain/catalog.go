package domain

import "time"

type SpecterScript struct {
	Identifier    string   `json:"identifier"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Documentation string   `json:"documentation"`
	ScriptBlock   string   `json:"scriptBlock"`
	Version       int      `json:"version"`
	IsPremium     bool     `json:"isPremium"`
	Labels        []string `json:"labels"`
	Enabled       bool     `json:"enabled"`
}

type AddWordlistRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Content     string   `json:"content"`
}

type WordlistResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Content     string    `json:"content"`
	EntryCount  int       `json:"entryCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Listener struct {
	Prefix          string `json:"prefix"`
	Enabled         bool   `json:"enabled"`
	CertificateName string `json:"certificateName"`
	WebDavPath      string `json:"webDavPath"`
}

type AddHTTPListenerRequest struct {
	Prefix          string `json:"prefix"`
	CertificateName string `json:"certificateName"`
	WebDavPath      string `json:"webDavPath"`
}

type UpdateListenerRequest struct {
	Prefix  string `json:"prefix"`
	Enabled bool   `json:"enabled"`
}

type CertificateInfo struct {
	Name               string    `json:"name"`
	Country            string    `json:"country"`
	State              string    `json:"state"`
	Locality           string    `json:"locality"`
	Organization       string    `json:"organization"`
	OrganizationalUnit string    `json:"organizationalUnit"`
	CommonName         string    `json:"commonName"`
	Start              time.Time `json:"start"`
	End                time.Time `json:"end"`
}

type SpecterModuleInfo struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	RuntimeVersion string   `json:"runtimeVersion"`
	Cmdlets        []string `json:"cmdlets"`
}
