package domain

import "time"

type NetHost struct {
	HostID         string    `json:"hostId"`
	Hostname       string    `json:"hostname"`
	Domain         string    `json:"domain"`
	OsName         string    `json:"osName"`
	OsVersion      string    `json:"osVersion"`
	OsArchitecture string    `json:"osArchitecture"`
	CidrNetwork    string    `json:"cidrNetwork"`
	FirstSeen      time.Time `json:"firstSeen"`
	LastSeen       time.Time `json:"lastSeen"`
}
type UpsertHostRequest struct {
	Hostname         string   `json:"hostname"`
	Domain           string   `json:"domain"`
	OsName           string   `json:"osName"`
	OsVersion        string   `json:"osVersion"`
	OsArchitecture   string   `json:"osArchitecture"`
	CidrNetwork      string   `json:"cidrNetwork"`
	KnownIpAddresses []string `json:"knownIpAddresses"`
}
type NetHostIPAddress struct {
	IpID          string `json:"ipId"`
	HostID        string `json:"hostId"`
	IpAddress     string `json:"ipAddress"`
	IsPrimary     bool   `json:"isPrimary"`
	InterfaceName string `json:"interfaceName"`
	MacAddress    string `json:"macAddress"`
	Vendor        string `json:"vendor"`
}
type UpsertIPAddressRequest struct {
	HostID        string `json:"hostId"`
	IpAddress     string `json:"ipAddress"`
	IsPrimary     bool   `json:"isPrimary"`
	InterfaceName string `json:"interfaceName"`
	MacAddress    string `json:"macAddress"`
}
type NetOpenPort struct {
	PortID      string    `json:"portId"`
	HostID      string    `json:"hostId"`
	PortNumber  int       `json:"portNumber"`
	Protocol    string    `json:"protocol"`
	State       string    `json:"state"`
	ServiceName string    `json:"serviceName"`
	Banner      string    `json:"banner"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
}
type UpsertOpenPortRequest struct {
	HostID      string `json:"hostId"`
	PortNumber  int    `json:"portNumber"`
	Protocol    string `json:"protocol"`
	State       string `json:"state"`
	ServiceName string `json:"serviceName"`
	Banner      string `json:"banner"`
}
