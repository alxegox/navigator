package k8s

import (
	"sync"

	"github.com/alxegox/navigator/pkg/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type Cache interface {
	UpdateService(namespace, name, clusterID, ClusterIP string, ports []Port, config NexusConfig) (updatedServiceKeys []QualifiedName)
	UpdateBackends(serviceName QualifiedName, clusterID string, endpointSetName QualifiedName, weight int, ips []string) (updatedServiceKeys []QualifiedName)
	UpdateConfig(services []QualifiedName, config NexusConfig) (updatedServiceKeys []QualifiedName)
	RemoveService(namespace, name string, clusterID string) (updatedServiceKeys []QualifiedName)
	RemoveBackends(serviceName QualifiedName, clusterID string, endpointSetName QualifiedName) (updatedServiceKeys []QualifiedName)
	FlushServiceByClusterID(serviceName QualifiedName, clusterID string)
	GetSnapshot() (services map[string]map[string]*Service)
}

type cache struct {
	mu       sync.RWMutex
	services map[QualifiedName]*Service
	logger   logrus.FieldLogger
	metrics  *observability.Metrics
}

func NewCache(logger logrus.FieldLogger, metrics *observability.Metrics) Cache {
	return &cache{
		services: make(map[QualifiedName]*Service),
		logger:   logger.WithField("context", "k8s.cache"),
		metrics:  metrics,
	}
}

func (c *cache) UpdateService(namespace, name, clusterID, clusterIP string, ports []Port, config NexusConfig) (updatedServiceKeys []QualifiedName) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := NewQualifiedName(namespace, name)
	service, ok := c.services[key]
	if !ok {
		service = NewService(namespace, name)
		c.services[key] = service
		c.metrics.ServiceCount.Inc()
	}

	updated := service.UpdateClusterIP(clusterID, clusterIP)
	updated = service.UpdatePorts(ports) || updated
	updated = service.UpdateConfig(config) || updated

	if updated {
		return []QualifiedName{key}
	}
	return nil
}

func (c *cache) UpdateConfig(services []QualifiedName, config NexusConfig) (updatedServiceKeys []QualifiedName) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, s := range services {
		service, ok := c.services[s]
		if !ok {
			continue
		}
		service.UpdateConfig(config)
		updatedServiceKeys = append(updatedServiceKeys, s)
	}
	return updatedServiceKeys
}

func (c *cache) UpdateBackends(serviceName QualifiedName, clusterID string, endpointSetName QualifiedName, weight int, ips []string) (updatedServiceKeys []QualifiedName) {
	c.mu.Lock()
	defer c.mu.Unlock()

	service, ok := c.services[serviceName]
	if !ok {
		service = NewService(serviceName.Namespace, serviceName.Name)
		c.services[serviceName] = service
		c.metrics.ServiceCount.Inc()
	}

	bsid := BackendSourceID{ClusterID: clusterID, EndpointSetName: endpointSetName}
	oldLen := len(service.BackendsBySourceID[bsid].AddrSet)

	updated := service.UpdateBackends(clusterID, endpointSetName, weight, ips)
	c.metrics.BackendsCount.With(prometheus.Labels{"service": serviceName.String(), "clusterID": clusterID}).Add(
		float64(len(service.BackendsBySourceID[bsid].AddrSet) - oldLen),
	)

	if updated {
		return []QualifiedName{serviceName}
	}

	return nil
}

func (c *cache) RemoveService(namespace, name string, clusterID string) (updatedServiceKeys []QualifiedName) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := NewQualifiedName(namespace, name)

	svc, ok := c.services[key]
	if !ok {
		return nil
	}
	svc.DeleteClusterIP(clusterID)
	c.metrics.BackendsCount.Delete(prometheus.Labels{"service": key.String(), "clusterID": clusterID})

	if len(svc.ClusterIPs) == 0 {
		delete(c.services, key)
		c.metrics.ServiceCount.Dec()
	}

	return []QualifiedName{key}
}

func (c *cache) RemoveBackends(serviceName QualifiedName, clusterID string, endpointSetName QualifiedName) (updatedServiceKeys []QualifiedName) {
	c.mu.Lock()
	defer c.mu.Unlock()

	service, ok := c.services[serviceName]
	if !ok {
		return nil
	}

	bsid := BackendSourceID{ClusterID: clusterID, EndpointSetName: endpointSetName}
	oldLen := len(service.BackendsBySourceID[bsid].AddrSet)

	updated := service.DeleteBackends(clusterID, endpointSetName)

	c.metrics.BackendsCount.With(prometheus.Labels{"service": serviceName.String(), "clusterID": clusterID}).Add(
		float64(len(service.BackendsBySourceID[bsid].AddrSet) - oldLen),
	)

	if updated {
		return []QualifiedName{serviceName}
	}

	return nil
}

// GetSnapshot returns services indexed by namespace and name: services[namespace][name]
func (c *cache) GetSnapshot() (services map[string]map[string]*Service) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	services = make(map[string]map[string]*Service)

	for i, service := range c.services {
		if _, ok := services[i.Namespace]; !ok {
			services[i.Namespace] = make(map[string]*Service)
		}
		services[i.Namespace][i.Name] = service.Copy()
	}

	return
}

func (c *cache) FlushServiceByClusterID(serviceName QualifiedName, clusterID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	service, ok := c.services[serviceName]
	if !ok {
		return
	}

	for backend := range service.BackendsBySourceID {
		if backend.ClusterID == clusterID {
			delete(service.BackendsBySourceID, backend)
		}
	}
}
