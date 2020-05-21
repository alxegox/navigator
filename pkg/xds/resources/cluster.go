package resources

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"

	api "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	cluster "github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoy_api_v2_endpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	envoyType "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"

	"github.com/alxegox/navigator/pkg/k8s"
)

const (
	clusterConnectionsLimit = 400000
	InboundIPv4ClusterName  = "InboundPassthroughClusterIpv4"
)

type ClusterCache struct {
	muUpdate sync.RWMutex

	DynamicConfig

	*BranchedResourceCache
}

func NewClusterCache(opts ...FuncOpt) *ClusterCache {
	c := &ClusterCache{
		BranchedResourceCache: NewBranchedResourceCache(cache.ClusterType),
	}
	c.DynamicClusterName = DefaultDynamicClusterName
	SetOpts(c, opts...)
	return c
}

func (c *ClusterCache) UpdateServices(updated, deleted []*k8s.Service) {
	c.muUpdate.Lock()
	defer c.muUpdate.Unlock()

	updatedResource := map[string][]NamedProtoMessage{}
	deletedResources := map[string][]NamedProtoMessage{}

	for _, u := range updated {
		c.getServiceClusters(updatedResource, u)
	}
	for _, d := range deleted {
		c.getServiceClusters(deletedResources, d)
	}

	c.UpdateBranchedResources(updatedResource, deletedResources)
}

func (c *ClusterCache) UpdateIngresses(updated map[k8s.QualifiedName]*k8s.Ingress) {
	// do nothing
}

func (c *ClusterCache) UpdateGateway(gw *k8s.Gateway) {
	// do nothing
}

func (c *ClusterCache) UpdateInboundPorts(ports []k8s.InboundPort) {
	c.muUpdate.Lock()
	defer c.muUpdate.Unlock()

	inboundClusters := []NamedProtoMessage{newGeneralInboundCluster()}
	for _, inboundPort := range ports {
		inboundClusters = append(inboundClusters, newDedicatedInboundCluster(inboundPort.Port))
	}

	c.SetDynamicResources(inboundClusters)
}

func (c *ClusterCache) getServiceClusters(storage map[string][]NamedProtoMessage, service *k8s.Service) {
	for _, p := range service.Ports {
		if p.Protocol != k8s.ProtocolTCP {
			continue
		}
		typ := getClusterType(p.Name)
		name := ClusterName(service.Namespace, service.Name, p.Name)
		clusterConf := newCluster(name, c.DynamicClusterName, service.CookieAffinity, typ)

		for _, clusterIP := range service.ClusterIPs {
			clusterID := clusterIP.ClusterID
			if _, ok := storage[clusterID]; !ok {
				storage[clusterID] = []NamedProtoMessage{}
			}
			storage[clusterID] = append(storage[clusterID], clusterConf)
		}
	}
}

// newCluster creates new v2.newCluster from v1.Service.
func newCluster(name, dynamicClusterName, cookieAffinity string, typ ClusterType) *api.Cluster {

	lbPolicy := api.Cluster_ROUND_ROBIN
	var lbConfig *api.Cluster_RingHashLbConfig_
	if cookieAffinity != "" && typ == HTTPCluster {
		lbPolicy = api.Cluster_RING_HASH
		lbConfig = &api.Cluster_RingHashLbConfig_{
			RingHashLbConfig: &api.Cluster_RingHashLbConfig{
				MinimumRingSize: &wrappers.UInt64Value{Value: 1024},
				HashFunction:    api.Cluster_RingHashLbConfig_XX_HASH,
			},
		}
	}

	return &api.Cluster{
		Name:                          name,
		ConnectTimeout:                &duration.Duration{Nanos: 250000000}, // 250ms = 250 000 000ns
		PerConnectionBufferLimitBytes: &wrappers.UInt32Value{Value: 32768},  // 32 Kb
		LbPolicy:                      lbPolicy,
		LbConfig:                      lbConfig,
		CommonLbConfig:                clusterCommonLBConfig(),
		CommonHttpProtocolOptions: &core.HttpProtocolOptions{
			IdleTimeout: &duration.Duration{Seconds: 3600},
		},
		HealthChecks: nil, // we use only passive health checking through Outlier detection
		EdsClusterConfig: &api.Cluster_EdsClusterConfig{
			EdsConfig:   configSource(dynamicClusterName),
			ServiceName: name,
		},
		ClusterDiscoveryType: &api.Cluster_Type{Type: api.Cluster_EDS},
		OutlierDetection: &cluster.OutlierDetection{
			// aggregation interval
			Interval: &duration.Duration{Seconds: 5},
			// we can eject all hosts (to move traffic to another DC)
			MaxEjectionPercent: &wrappers.UInt32Value{Value: 100},
			// host is ejected for this amount of time
			BaseEjectionTime: &duration.Duration{Seconds: 10},
			// number of consecutive 502,503,504 or network errors such as connect error
			ConsecutiveGatewayFailure: &wrappers.UInt32Value{Value: 30},
			// turn on consecutive gateway outlier logic
			EnforcingConsecutiveGatewayFailure: &wrappers.UInt32Value{Value: 100},
			// turn off consecutive 5xx outlier logic
			EnforcingConsecutive_5Xx: &wrappers.UInt32Value{Value: 0},
			// turn off success rate outlier logic
			EnforcingSuccessRate: &wrappers.UInt32Value{Value: 0},
		},
		CircuitBreakers: &cluster.CircuitBreakers{
			Thresholds: []*cluster.CircuitBreakers_Thresholds{
				{
					Priority:           core.RoutingPriority_DEFAULT,
					MaxConnections:     &wrappers.UInt32Value{Value: clusterConnectionsLimit},
					MaxRequests:        &wrappers.UInt32Value{Value: clusterConnectionsLimit},
					MaxPendingRequests: &wrappers.UInt32Value{Value: clusterConnectionsLimit},
				},
				{
					Priority:           core.RoutingPriority_HIGH,
					MaxConnections:     &wrappers.UInt32Value{Value: clusterConnectionsLimit},
					MaxRequests:        &wrappers.UInt32Value{Value: clusterConnectionsLimit},
					MaxPendingRequests: &wrappers.UInt32Value{Value: clusterConnectionsLimit},
				},
			},
		},
	}
}

// newInboundCluster returns inbound cluster for passing through any traffic
func newGeneralInboundCluster() *api.Cluster {
	return &api.Cluster{
		Name:           InboundIPv4ClusterName,
		ConnectTimeout: &duration.Duration{Seconds: 1},
		LbPolicy:       api.Cluster_CLUSTER_PROVIDED,
		//HealthChecks: nil, // we use only passive health checking through Outlier detection
		ClusterDiscoveryType: &api.Cluster_Type{Type: api.Cluster_ORIGINAL_DST},
		// turn off all limits
		CircuitBreakers: &cluster.CircuitBreakers{
			Thresholds: []*cluster.CircuitBreakers_Thresholds{{
				MaxConnections:     &wrappers.UInt32Value{Value: 4294967295},
				MaxPendingRequests: &wrappers.UInt32Value{Value: 4294967295},
				MaxRequests:        &wrappers.UInt32Value{Value: 4294967295},
				MaxRetries:         &wrappers.UInt32Value{Value: 4294967295},
			},
			},
		},
		UpstreamBindConfig: &core.BindConfig{
			SourceAddress: &core.SocketAddress{
				Address: "127.0.0.6", // magic address
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: 0,
				},
			},
		},
	}
}

// newDedicatedInboundCluster creates cluster for particular inbound port (to track traffic)
func newDedicatedInboundCluster(port int) *api.Cluster {
	return &api.Cluster{
		Name:                 inboundClusterName(port),
		ConnectTimeout:       &duration.Duration{Seconds: 1},
		LbPolicy:             api.Cluster_ROUND_ROBIN,
		ClusterDiscoveryType: &api.Cluster_Type{Type: api.Cluster_STATIC},
		// turn off all limits
		CircuitBreakers: &cluster.CircuitBreakers{
			Thresholds: []*cluster.CircuitBreakers_Thresholds{{
				MaxConnections:     &wrappers.UInt32Value{Value: 4294967295},
				MaxPendingRequests: &wrappers.UInt32Value{Value: 4294967295},
				MaxRequests:        &wrappers.UInt32Value{Value: 4294967295},
				MaxRetries:         &wrappers.UInt32Value{Value: 4294967295},
			},
			},
		},
		LoadAssignment: &api.ClusterLoadAssignment{
			ClusterName: inboundClusterName(port),
			Endpoints: []*envoy_api_v2_endpoint.LocalityLbEndpoints{{
				LbEndpoints: []*envoy_api_v2_endpoint.LbEndpoint{lbEndpoint("127.0.0.1", port, 1)},
			}},
		},
	}
}

func configSource(cluster string) *core.ConfigSource {
	return &core.ConfigSource{
		ConfigSourceSpecifier: &core.ConfigSource_Ads{
			Ads: &core.AggregatedConfigSource{},
		},
	}
}

// clusterCommonLBConfig creates a *v2.Cluster_CommonLbConfig with HealthyPanicThreshold disabled.
func clusterCommonLBConfig() *api.Cluster_CommonLbConfig {
	return &api.Cluster_CommonLbConfig{
		HealthyPanicThreshold: &envoyType.Percent{ // Disable HealthyPanicThreshold
			Value: 0,
		},
	}
}

type ClusterType string

const (
	HTTPCluster ClusterType = "http"
	TCPCluster  ClusterType = "tcp"
)

func getClusterType(portName string) ClusterType {
	tokens := strings.Split(portName, "-")
	if len(tokens) == 0 {
		return TCPCluster
	}
	if tokens[0] == "http" {
		return HTTPCluster
	}
	return TCPCluster
}

func inboundClusterName(port int) string {
	return fmt.Sprintf("inbound_%d", port)
}

// ClusterName returns the name of the CDS cluster for this service.
func ClusterName(ns, name, portName string) string {
	if portName == "" {
		return hashname(60, ns, name)
	}
	return hashname(60, ns, name, portName)
}

// hashname takes a lenth l and a varargs of strings s and returns a string whose length
// which does not exceed l. Internally s is joined with strings.Join(s, "/"). If the
// combined length exceeds l then hashname truncates each element in s, starting from the
// end using a hash derived from the contents of s (not the current element). This process
// continues until the length of s does not exceed l, or all elements have been truncated.
// In which case, the entire string is replaced with a hash not exceeding the length of l.
func hashname(l int, s ...string) string {
	const shorthash = 6 // the length of the shorthash

	r := strings.Join(s, "/")
	if l > len(r) {
		// we're under the limit, nothing to do
		return r
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(r)))
	for n := len(s) - 1; n >= 0; n-- {
		s[n] = truncate(l/len(s), s[n], hash[:shorthash])
		r = strings.Join(s, "/")
		if l > len(r) {
			return r
		}
	}
	// truncated everything, but we're still too long
	// just return the hash truncated to l.
	return hash[:min(len(hash), l)]
}

// truncate truncates s to l length by replacing the
// end of s with -suffix.
func truncate(l int, s, suffix string) string {
	if l >= len(s) {
		// under the limit, nothing to do
		return s
	}
	if l <= len(suffix) {
		// easy case, just return the start of the suffix
		return suffix[:min(l, len(suffix))]
	}
	return s[:l-len(suffix)-1] + "-" + suffix
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}
