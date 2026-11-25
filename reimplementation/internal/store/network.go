package store

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/research/specter-reimplementation/internal/domain"
)

func (s *Store) NetHosts(ctx context.Context) ([]domain.NetHost, error) {
	rows, err := s.pool.Query(ctx, `SELECT hostid,COALESCE(hostname,''),COALESCE(domain,''),COALESCE(osname,''),
		COALESCE(osversion,''),COALESCE(osarchitecture,''),COALESCE(cidrnetwork,''),firstseen,lastseen FROM nethosts ORDER BY hostname,hostid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.NetHost{}
	for rows.Next() {
		var value domain.NetHost
		if err := rows.Scan(&value.HostID, &value.Hostname, &value.Domain, &value.OsName, &value.OsVersion, &value.OsArchitecture, &value.CidrNetwork, &value.FirstSeen, &value.LastSeen); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) NetHost(ctx context.Context, hostID string) (domain.NetHost, error) {
	var value domain.NetHost
	err := s.pool.QueryRow(ctx, `SELECT hostid,COALESCE(hostname,''),COALESCE(domain,''),COALESCE(osname,''),COALESCE(osversion,''),COALESCE(osarchitecture,''),COALESCE(cidrnetwork,''),firstseen,lastseen FROM nethosts WHERE hostid=$1`, hostID).Scan(&value.HostID, &value.Hostname, &value.Domain, &value.OsName, &value.OsVersion, &value.OsArchitecture, &value.CidrNetwork, &value.FirstSeen, &value.LastSeen)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NetHost{}, ErrNotFound
	}
	return value, err
}

func (s *Store) UpsertNetHost(ctx context.Context, request domain.UpsertHostRequest) (domain.NetHost, error) {
	if strings.TrimSpace(request.Hostname) == "" || len(request.Hostname) > 253 || len(request.Domain) > 253 || len(request.OsName) > 256 || len(request.OsVersion) > 256 || len(request.OsArchitecture) > 64 || len(request.KnownIpAddresses) > 64 {
		return domain.NetHost{}, errors.New("invalid network host")
	}
	if request.CidrNetwork != "" {
		if _, _, err := net.ParseCIDR(request.CidrNetwork); err != nil {
			return domain.NetHost{}, errors.New("invalid CIDR network")
		}
	}
	for _, ip := range request.KnownIpAddresses {
		if net.ParseIP(ip) == nil {
			return domain.NetHost{}, errors.New("invalid host IP address")
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.NetHost{}, err
	}
	defer tx.Rollback(ctx)
	var hostID string
	err = tx.QueryRow(ctx, `SELECT hostid FROM nethosts WHERE lower(COALESCE(hostname,''))=lower($1) AND lower(COALESCE(domain,''))=lower($2) ORDER BY id LIMIT 1`, request.Hostname, request.Domain).Scan(&hostID)
	if errors.Is(err, pgx.ErrNoRows) {
		hostID = randomStoreID("host")
		_, err = tx.Exec(ctx, `INSERT INTO nethosts(hostid,hostname,domain,osname,osversion,osarchitecture,cidrnetwork,firstseen,lastseen) VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),now(),now())`, hostID, request.Hostname, request.Domain, request.OsName, request.OsVersion, request.OsArchitecture, request.CidrNetwork)
	} else if err == nil {
		_, err = tx.Exec(ctx, `UPDATE nethosts SET osname=NULLIF($2,''),osversion=NULLIF($3,''),osarchitecture=NULLIF($4,''),cidrnetwork=NULLIF($5,''),lastseen=now() WHERE hostid=$1`, hostID, request.OsName, request.OsVersion, request.OsArchitecture, request.CidrNetwork)
	}
	if err != nil {
		return domain.NetHost{}, err
	}
	for index, ip := range request.KnownIpAddresses {
		_, err = tx.Exec(ctx, `INSERT INTO nethostipaddresses(ipid,hostid,ipaddress,isprimary) VALUES($1,$2,$3,$4) ON CONFLICT(hostid,ipaddress) DO UPDATE SET isprimary=excluded.isprimary`, randomStoreID("ip"), hostID, ip, index == 0)
		if err != nil {
			return domain.NetHost{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.NetHost{}, err
	}
	return s.NetHost(ctx, hostID)
}

func (s *Store) DeleteNetHost(ctx context.Context, hostID string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM nethosts WHERE hostid=$1", hostID)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) NetHostIPs(ctx context.Context, hostID string) ([]domain.NetHostIPAddress, error) {
	rows, err := s.pool.Query(ctx, `SELECT ipid,hostid,ipaddress,isprimary,COALESCE(interfacename,''),COALESCE(macaddress,'') FROM nethostipaddresses WHERE hostid=$1 ORDER BY isprimary DESC,ipaddress`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.NetHostIPAddress{}
	for rows.Next() {
		var value domain.NetHostIPAddress
		if err := rows.Scan(&value.IpID, &value.HostID, &value.IpAddress, &value.IsPrimary, &value.InterfaceName, &value.MacAddress); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) UpsertNetHostIP(ctx context.Context, value domain.UpsertIPAddressRequest) error {
	if !validCatalogID(value.HostID) || net.ParseIP(value.IpAddress) == nil || len(value.InterfaceName) > 256 || len(value.MacAddress) > 64 {
		return errors.New("invalid host IP address")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO nethostipaddresses(ipid,hostid,ipaddress,isprimary,interfacename,macaddress) VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')) ON CONFLICT(hostid,ipaddress) DO UPDATE SET isprimary=excluded.isprimary,interfacename=excluded.interfacename,macaddress=excluded.macaddress`, randomStoreID("ip"), value.HostID, value.IpAddress, value.IsPrimary, value.InterfaceName, value.MacAddress)
	return err
}

func (s *Store) NetOpenPorts(ctx context.Context, hostID string) ([]domain.NetOpenPort, error) {
	rows, err := s.pool.Query(ctx, `SELECT portid,hostid,portnumber,protocol,state,COALESCE(servicename,''),COALESCE(banner,''),firstseen,lastseen FROM netopenports WHERE hostid=$1 ORDER BY portnumber,protocol`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.NetOpenPort{}
	for rows.Next() {
		var value domain.NetOpenPort
		if err := rows.Scan(&value.PortID, &value.HostID, &value.PortNumber, &value.Protocol, &value.State, &value.ServiceName, &value.Banner, &value.FirstSeen, &value.LastSeen); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) UpsertNetOpenPort(ctx context.Context, value domain.UpsertOpenPortRequest) error {
	value.Protocol = strings.ToLower(value.Protocol)
	if !validCatalogID(value.HostID) || value.PortNumber < 1 || value.PortNumber > 65535 || (value.Protocol != "tcp" && value.Protocol != "udp") || len(value.State) > 64 || len(value.ServiceName) > 256 || len(value.Banner) > 4096 {
		return errors.New("invalid open port")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO netopenports(portid,hostid,portnumber,protocol,state,servicename,banner,firstseen,lastseen) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),now(),now()) ON CONFLICT(hostid,portnumber,protocol) DO UPDATE SET state=excluded.state,servicename=excluded.servicename,banner=excluded.banner,lastseen=now()`, randomStoreID("port"), value.HostID, value.PortNumber, value.Protocol, value.State, value.ServiceName, value.Banner)
	return err
}

func (s *Store) ResolveNetHostID(ctx context.Context, fqdn, ip string) (string, error) {
	var hostID string
	err := s.pool.QueryRow(ctx, `SELECT h.hostid FROM nethosts h LEFT JOIN nethostipaddresses i ON i.hostid=h.hostid WHERE (lower(concat_ws('.',h.hostname,h.domain))=lower($1) AND $1<>'') OR (i.ipaddress=$2 AND $2<>'') ORDER BY h.id LIMIT 1`, fqdn, ip).Scan(&hostID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return hostID, err
}
