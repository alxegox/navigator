package k8s

import (
	"regexp"
	"sync"

	navigatorV1 "github.com/alxegox/navigator/pkg/apis/navigator/v1"
)

type NexusCache interface {
	Delete(cluster string, nexusResource *navigatorV1.Nexus) (updatedAppNames []string)
	Update(cluster string, nexusResource *navigatorV1.Nexus) (updatedAppNames []string)
	GetSnapshotByAppName(appNames []string) (nexusesByAppName map[string]Nexus)
	GetAppNamesSnapshotByService(servicesInvolved []QualifiedName) (appNamesByService map[QualifiedName][]string)
	GetConfigByService(namespace, name string) NexusConfig
	GetNexus(appName string) Nexus
}

type nexusCache struct {
	mu                  sync.RWMutex
	nexusesByAppName    map[string]*partitionedNexus
	appNamesByService   map[QualifiedName][]string
	appNameByDownstream map[QualifiedName]string
}

func NewNexusCache() *nexusCache {
	return &nexusCache{
		nexusesByAppName:  make(map[string]*partitionedNexus),
		appNamesByService: make(map[QualifiedName][]string),
	}
}

// Update updates cache if passed nexus is different from nexus in cache
func (c *nexusCache) Update(cluster string, nexusResource *navigatorV1.Nexus) []string {
	appName := nexusResource.Spec.AppName

	c.mu.Lock()
	defer c.mu.Unlock()

	_, ok := c.nexusesByAppName[appName]
	if !ok {
		c.nexusesByAppName[appName] = newPartitionNexus(appName)
	}
	updated := c.nexusesByAppName[appName].UpdateServices(cluster, nexusResource)
	if !updated {
		return nil
	}
	c.renewAppNamesByService()
	return []string{appName}
}

func (c *nexusCache) Delete(cluster string, nexusResource *navigatorV1.Nexus) []string {
	appName := nexusResource.Spec.AppName
	namespace := nexusResource.Namespace
	name := nexusResource.Name

	c.mu.Lock()
	defer c.mu.Unlock()

	_, ok := c.nexusesByAppName[appName]
	if !ok {
		return nil
	}
	updated := c.nexusesByAppName[appName].RemoveServices(cluster, namespace, name)
	if !updated {
		return nil
	}
	if c.nexusesByAppName[appName].isEmpty() {
		delete(c.nexusesByAppName, appName)
	}

	c.renewAppNamesByService()
	return []string{appName}
}

func (c *nexusCache) renewAppNamesByService() {
	resultService := make(map[QualifiedName][]string)
	resultDownstream := make(map[QualifiedName]string)
	for appName, appPartitionedNexus := range c.nexusesByAppName {
		for _, serviceKey := range appPartitionedNexus.Nexus.Services {
			resultService[serviceKey] = append(resultService[serviceKey], appName)
		}
		for _, downstreamKey := range appPartitionedNexus.Nexus.Downstreams {
			resultDownstream[downstreamKey] = appName
		}
	}
	c.appNamesByService = resultService
	c.appNameByDownstream = resultDownstream
}

// GetSnapshotByAppName returns nexus snapshot filtered by appNames
func (c *nexusCache) GetSnapshotByAppName(appNames []string) map[string]Nexus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nexusesByAppName := make(map[string]Nexus)
	if len(appNames) > 0 {
		for k, v := range c.nexusesByAppName {
			found := false
			for _, appName := range appNames {
				if k == appName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			nexusesByAppName[k] = v.Nexus
		}
	}

	return nexusesByAppName
}

func (c *nexusCache) GetConfigByService(namespace, name string) NexusConfig {
	service := QualifiedName{
		Namespace: namespace,
		Name:      name,
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	appName, ok := c.appNameByDownstream[service]
	if !ok {
		return defaultConfig
	}
	nx, ok := c.nexusesByAppName[appName]
	if !ok {
		return defaultConfig
	}
	return nx.Nexus.NexusConfig
}

func (c *nexusCache) GetAppNamesSnapshotByService(servicesInvolved []QualifiedName) map[QualifiedName][]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	appNamesByService := make(map[QualifiedName][]string)

	if len(servicesInvolved) == 0 {
		return appNamesByService
	}

	for k, v := range c.appNamesByService {
		found := false
		for _, service := range servicesInvolved {
			if k == service {
				found = true
				break
			}
			// check whether nexus is regexp
			if k.IsNSRegexp() {
				matched, _ := regexp.MatchString(k.Namespace, service.Namespace)
				if matched {
					found = true
					break
				}
			}
		}
		if !found {
			continue
		}
		appNamesByService[k] = append([]string{}, v...)
	}

	return appNamesByService
}

func (c *nexusCache) GetNexus(appName string) Nexus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	partNx, ok := c.nexusesByAppName[appName]
	if !ok {
		return NewNexus(appName, nil, nil, nil)
	}
	return partNx.Nexus
}
