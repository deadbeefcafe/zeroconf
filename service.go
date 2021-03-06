package zeroconf

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ServiceRecord contains the basic description of a service, which contains instance name, service type & domain
type ServiceRecord struct {
	Instance string `json:"name"`   // Instance name (e.g. "My web page")
	Service  string `json:"type"`   // Service name (e.g. _http._tcp.)
	Domain   string `json:"domain"` // If blank, assumes "local"

	// private variable populated on the first call to ServiceName()/ServiceInstanceName()
	serviceName         string
	serviceInstanceName string
	serviceTypeName     string
}

// ServiceName returns a complete service name (e.g. _foobar._tcp.local.), which is composed
// of a service name (also referred as service type) and a domain.
func (s *ServiceRecord) ServiceName() string {
	if s.serviceName == "" {
		s.serviceName = fmt.Sprintf("%s.%s.", trimDot(s.Service), trimDot(s.Domain))
	}
	return s.serviceName
}

// ServiceInstanceName returns a complete service instance name (e.g. MyDemo\ Service._foobar._tcp.local.),
// which is composed from service instance name, service name and a domain.
func (s *ServiceRecord) ServiceInstanceName() string {
	// If no instance name provided we cannot compose service instance name
	if s.Instance == "" {
		return ""
	}
	// If not cached - compose and cache
	if s.serviceInstanceName == "" {
		s.serviceInstanceName = fmt.Sprintf("%s.%s", trimDot(s.Instance), s.ServiceName())
	}
	return s.serviceInstanceName
}

// ServiceTypeName returns the complete identifier for a DNS-SD query.
func (s *ServiceRecord) ServiceTypeName() string {
	// If not cached - compose and cache
	if s.serviceTypeName == "" {
		domain := "local"
		if len(s.Domain) > 0 {
			domain = trimDot(s.Domain)
		}
		s.serviceTypeName = fmt.Sprintf("_services._dns-sd._udp.%s.", domain)
	}
	return s.serviceTypeName
}

// NewServiceRecord constructs a ServiceRecord.
func NewServiceRecord(instance, service, domain string) *ServiceRecord {
	return &ServiceRecord{instance, service, domain, "", "", ""}
}

// LookupParams contains configurable properties to create a service discovery request
type LookupParams struct {
	ServiceRecord
	Entries     chan<- *ServiceEntry // Entries Channel
	stopProbing chan struct{}
	once        sync.Once
}

// NewLookupParams constructs a LookupParams.
func NewLookupParams(instance, service, domain string, entries chan<- *ServiceEntry) *LookupParams {
	return &LookupParams{
		ServiceRecord: *NewServiceRecord(instance, service, domain),
		Entries:       entries,

		stopProbing: make(chan struct{}),
	}
}

// Notify subscriber that no more entries will arrive. Mostly caused
// by an expired context.
func (l *LookupParams) done() {
	if l.Entries != nil {
		close(l.Entries)
	}
}

func (l *LookupParams) disableProbing() {
	l.once.Do(func() { close(l.stopProbing) })
}

// ServiceEventType is type of event happening on a service
type ServiceEventType string

const (
	// NewOrUpdated service instance
	NewOrUpdated ServiceEventType = "NewOrUpdated"
	// Removed notifies when an instance is going away
	Removed ServiceEventType = "Removed"
)

// ServiceEntry represents a browse/lookup result for client API.
// It is also used to configure service registration (server API), which is
// used to answer multicast queries.
type ServiceEntry struct {
	ServiceRecord
	HostName string   `json:"hostname"` // Host machine DNS name
	Port     int      `json:"port"`     // Service Port
	Text     []string `json:"text"`     // Service info served as a TXT record
	TTL      uint32   `json:"ttl"`      // TTL of the service record
	AddrIPv4 []net.IP `json:"-"`        // Host machine IPv4 address
	AddrIPv6 []net.IP `json:"-"`        // Host machine IPv6 address

	eventType   ServiceEventType // whether its removed or updated
	refreshTime *time.Time       // when to refresh entry before the cache expiry
	expiryTime  *time.Time       // when to expire entry and delete from cache
}

// EventType returns the type of event that happened to ServiceEntry
func (s *ServiceEntry) EventType() ServiceEventType {
	return s.eventType
}

type entryCache struct {
	sync.RWMutex
	entries map[string]*ServiceEntry
}

// Browser is reference to an instance of browser
type Browser struct {
	cache *entryCache
}

// Entries returns the current cache
func (b *Browser) Entries() []ServiceEntry {
	if b.cache != nil {
		var r []ServiceEntry
		b.cache.RLock()
		for _, v := range b.cache.entries {
			r = append(r, *v)
		}
		b.cache.RUnlock()
		return r
	}
	return nil
}

// NewServiceEntry constructs a ServiceEntry.
func NewServiceEntry(instance, service, domain string) *ServiceEntry {
	return &ServiceEntry{
		ServiceRecord: *NewServiceRecord(instance, service, domain),
	}
}
