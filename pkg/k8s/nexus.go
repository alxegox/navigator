package k8s

import (
	navigatorV1 "github.com/alxegox/navigator/pkg/apis/navigator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var defaultConfig = NexusConfig{}

type InboundPort struct {
	Name string
	Port int
}

type NexusConfig struct {
	CookieAffinity string
}

type Nexus struct {
	NexusConfig

	AppName      string
	Services     []QualifiedName // if name is empty, allow access to all services in the namespace
	Downstreams  []QualifiedName // if slice is empty, allow access to all services in the AppName namespace
	InboundPorts []InboundPort
}

func NewNexus(appName string, services, downstreams []QualifiedName, inboundPorts []InboundPort) Nexus {
	return Nexus{
		NexusConfig:  defaultConfig,
		AppName:      appName,
		Services:     append([]QualifiedName{}, services...),
		Downstreams:  append([]QualifiedName{}, downstreams...),
		InboundPorts: append([]InboundPort{}, inboundPorts...),
	}
}

func (n Nexus) isEqual(other Nexus) bool {
	if n.AppName != other.AppName || n.CookieAffinity != other.CookieAffinity {
		return false
	}
	if len(n.Services) != len(other.Services) ||
		len(n.Downstreams) != len(other.Downstreams) ||
		len(n.InboundPorts) != len(other.InboundPorts) {
		return false
	}

	//todo: implement set to prevent disorder
	servicesSet := map[QualifiedName]struct{}{}
	for _, s := range other.Services {
		servicesSet[s] = struct{}{}
	}
	for _, name := range n.Services {
		if _, ok := servicesSet[name]; !ok {
			return false
		}
	}

	for i, name := range n.Downstreams {
		if name != other.Downstreams[i] {
			return false
		}
	}

	for i, inboundPort := range n.InboundPorts {
		if inboundPort != other.InboundPorts[i] {
			return false
		}
	}

	return true
}

// partitionedNexus designed to hold services from multiple Nexus custom resource instances,
// where every instance has unique partition ID (actually, CR Nexus.Name) to correctly process updates/deletes of partitions
// example:
// moment zero: we have Nexus CR {Name: helmgen-v1, services: {test-1, test-2}. resulting navigator nexus has services={test-1, test-2}
// moment 1: appears Nexus CR {Name: helmgen-v2, services: {test-2, test-3}}.  resulting navigator nexus has services={test-1, test-2, test-3}
// moment 2:  Nexus CR "helmgen-v1" DISAPPEARS, still only "helmgen-v2".  resulting navigator nexus has services={test-2, test-3}
type partitionedNexus struct {
	AppName string
	Nexus   Nexus

	parts map[partKey]*navigatorV1.Nexus
}

type partKey struct {
	Cluster   string
	Namespace string
	Name      string
}

func newPartKey(cluster, namespace, name string) partKey {
	return partKey{
		Cluster:   cluster,
		Namespace: namespace,
		Name:      name,
	}
}

func newPartitionNexus(appName string) *partitionedNexus {
	return &partitionedNexus{
		AppName: appName,
		parts:   make(map[partKey]*navigatorV1.Nexus),
	}
}

func (pn *partitionedNexus) UpdateServices(cluster string, nexusResource *navigatorV1.Nexus) (updated bool) {
	if nexusResource.Spec.AppName != pn.AppName {
		return false
	}
	key := newPartKey(cluster, nexusResource.Namespace, nexusResource.Name)
	pn.parts[key] = nexusResource
	return pn.recreateNexus()
}

func (pn *partitionedNexus) RemoveServices(cluster, namespace, name string) (updated bool) {
	key := newPartKey(cluster, namespace, name)
	if _, ok := pn.parts[key]; !ok {
		return false
	}
	delete(pn.parts, key)
	return pn.recreateNexus()
}

func (pn *partitionedNexus) recreateNexus() (updated bool) {
	newNexus := pn.generateNexus()
	isEqual := pn.Nexus.isEqual(newNexus)
	if isEqual {
		return false
	}
	pn.Nexus = newNexus
	return true
}

// generateNexus merges services from all partitions into one Nexus
func (pn *partitionedNexus) generateNexus() Nexus {
	nexus := Nexus{
		AppName:     pn.AppName,
		NexusConfig: defaultConfig,
	}
	if len(pn.parts) == 0 {
		return nexus
	}

	// 1. analysis
	var maxNexus *navigatorV1.Nexus
	serviceSet := map[QualifiedName]struct{}{}
	inboundPortsSet := map[int]InboundPort{}
	downstreamSet := map[QualifiedName]struct{}{}

	for _, nexusPart := range pn.parts {
		for _, service := range nexusPart.Spec.Services {
			serviceKey := QualifiedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}
			serviceSet[serviceKey] = struct{}{}
		}
		for _, downstream := range nexusPart.Spec.Downstreams {
			serviceKey := QualifiedName{
				Name:      downstream.Name,
				Namespace: downstream.Namespace,
			}
			downstreamSet[serviceKey] = struct{}{}
		}
		for _, inboundPort := range nexusPart.Spec.InboundPorts {
			inboundPortsSet[inboundPort.Port] = InboundPort{inboundPort.Name, inboundPort.Port}
		}
		if maxNexus == nil || maxNexus.Name < nexusPart.Name {
			maxNexus = nexusPart
		}
	}

	// 2. syntez
	for service := range serviceSet {
		nexus.Services = append(nexus.Services, service)
	}
	for downstream := range downstreamSet {
		nexus.Downstreams = append(nexus.Downstreams, downstream)
	}
	for _, inboundPort := range inboundPortsSet {
		nexus.InboundPorts = append(nexus.InboundPorts, inboundPort)
	}
	nexus.CookieAffinity = maxNexus.Spec.CookieAffinity

	return nexus
}

func (pn *partitionedNexus) isEmpty() bool {
	return len(pn.parts) == 0
}

func NewVirtualNexus(namespace, name, appName string, services []QualifiedName) *navigatorV1.Nexus {

	naviServices := make([]navigatorV1.Service, 0, len(services))
	for _, s := range services {
		naviServices = append(naviServices, navigatorV1.Service{
			Name:      s.Name,
			Namespace: s.Namespace,
		})
	}

	return &navigatorV1.Nexus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: navigatorV1.NexusSpec{
			AppName:  appName,
			Services: naviServices,
		},
	}
}
