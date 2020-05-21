package resources

import (
	"fmt"
	"net"
	"sync"

	api "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	http "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	tcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"

	"github.com/alxegox/navigator/pkg/k8s"
)

const (
	GatewayListenerPrefix = "~~gateway"
	VirtualInboundPrefix  = "~~virtual_inbound"
)

type ListenerCache struct {
	muUpdate sync.RWMutex

	DynamicConfig
	HealthCheckConfig
	InboundPortConfig

	*BranchedResourceCache
	isGatewayOnly bool
}

func NewListenerCache(opts ...FuncOpt) *ListenerCache {
	l := &ListenerCache{
		BranchedResourceCache: NewBranchedResourceCache(cache.ListenerType),
	}

	l.DynamicClusterName = DefaultDynamicClusterName
	SetOpts(l, opts...)

	static := []NamedProtoMessage{}
	if l.EnableHealthCheck {
		static = append(static, l.healthListener())
	}
	l.SetStaticResources(static)
	return l
}

func (c *ListenerCache) UpdateServices(updated, deleted []*k8s.Service) {

	c.muUpdate.Lock()
	defer c.muUpdate.Unlock()

	if c.isGatewayOnly {
		return
	}

	updatedResource := map[string][]NamedProtoMessage{}
	deletedResources := map[string][]NamedProtoMessage{}

	for _, u := range updated {
		c.getServiceListeners(updatedResource, u)
	}
	for _, d := range deleted {
		c.getServiceListeners(deletedResources, d)
	}

	c.UpdateBranchedResources(updatedResource, deletedResources)
}

func (c *ListenerCache) UpdateIngresses(updated map[k8s.QualifiedName]*k8s.Ingress) {
	// do nothing
}

func (c *ListenerCache) UpdateGateway(gw *k8s.Gateway) {
	c.muUpdate.Lock()
	defer c.muUpdate.Unlock()

	if !c.isGatewayOnly {
		c.isGatewayOnly = true
		c.RenewResourceCache()
	}
	updatedResource := map[string][]NamedProtoMessage{}
	c.getGatewayListeners(updatedResource, gw)
	c.UpdateBranchedResources(updatedResource, nil)
}

func (c *ListenerCache) UpdateInboundPorts(ports []k8s.InboundPort) {
	c.muUpdate.Lock()
	defer c.muUpdate.Unlock()
	if c.isGatewayOnly {
		return
	}
	inboundListeners := []NamedProtoMessage{virtualInboundListener(c.InboundPort, ports)}
	c.SetDynamicResources(inboundListeners)
}

func (c *ListenerCache) healthListener() *api.Listener {
	return &api.Listener{
		Name:                          c.HealthCheckName,
		Address:                       socketAddress("0.0.0.0", c.HealthCheckPort),
		PerConnectionBufferLimitBytes: &wrappers.UInt32Value{Value: 32768}, // 32 Kb
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: makeAny(&http.HttpConnectionManager{
						NormalizePath:     &wrappers.BoolValue{Value: true},
						MergeSlashes:      true,
						GenerateRequestId: &wrappers.BoolValue{Value: false},
						StreamIdleTimeout: &duration.Duration{Seconds: 300},
						StatPrefix:        c.HealthCheckName,
						HttpFilters: []*http.HttpFilter{{
							Name: wellknown.Router,
						}},
						RouteSpecifier: &http.HttpConnectionManager_Rds{
							Rds: &http.Rds{
								RouteConfigName: c.HealthCheckName,
								ConfigSource:    configSource(c.DynamicClusterName),
							},
						},
					}),
				},
			}},
		}},
	}
}

func (c *ListenerCache) getGatewayListeners(storage map[string][]NamedProtoMessage, gateway *k8s.Gateway) {
	clusterID := gateway.ClusterID
	if _, ok := storage[clusterID]; !ok {
		storage[clusterID] = []NamedProtoMessage{}
	}
	if len(gateway.Listens) > 0 {
		for _, v := range gateway.Listens {
			listenerConf := gatewayListener(v.Address, v.Port, c.DynamicClusterName)
			storage[clusterID] = append(storage[clusterID], listenerConf)
		}
	} else {
		listenerConf := gatewayListener("0.0.0.0", gateway.Port, c.DynamicClusterName)
		storage[clusterID] = append(storage[clusterID], listenerConf)
	}
}

func (c *ListenerCache) getServiceListeners(storage map[string][]NamedProtoMessage, service *k8s.Service) {
	for _, p := range service.Ports {
		if p.Protocol != k8s.ProtocolTCP {
			continue
		}
		for _, clusterIP := range service.ClusterIPs {
			ip := net.ParseIP(clusterIP.IP)
			// skip invalid ip addr
			// e.g. "None" from k8s static service
			if ip == nil {
				continue
			}

			clusterID := clusterIP.ClusterID
			var clusterListener *api.Listener
			switch getClusterType(p.Name) {
			case HTTPCluster:
				clusterListener = httpListener(clusterIP, p, service, c.DynamicClusterName)
			case TCPCluster:
				clusterListener = tcpListener(clusterIP, p, service)
			}

			if _, ok := storage[clusterID]; !ok {
				storage[clusterID] = []NamedProtoMessage{}
			}
			storage[clusterID] = append(storage[clusterID], clusterListener)
		}
	}
}

func tcpListener(clusterIP k8s.Address, port k8s.Port, service *k8s.Service) *api.Listener {
	name := ListenerNameFromService(service.Namespace, service.Name, clusterIP.IP, port.Port)
	prefix := ListenerStatPrefix(service.Namespace, service.Name, port.Port)

	return &api.Listener{
		Name:             name,
		Address:          socketAddress(clusterIP.IP, port.Port),
		TrafficDirection: core.TrafficDirection_OUTBOUND,
		DeprecatedV1: &api.Listener_DeprecatedV1{
			BindToPort: &wrappers.BoolValue{
				Value: false,
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.TCPProxy,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: makeAny(&tcp.TcpProxy{
						StatPrefix: prefix,
						ClusterSpecifier: &tcp.TcpProxy_Cluster{
							Cluster: ClusterName(service.Namespace, service.Name, port.Name),
						},
					}),
				},
			}},
		}},
	}
}

func httpListener(clusterIP k8s.Address, port k8s.Port, service *k8s.Service, dynamicClusterName string) *api.Listener {
	name := ListenerNameFromService(service.Namespace, service.Name, clusterIP.IP, port.Port)
	prefix := ListenerStatPrefix(service.Namespace, service.Name, port.Port)

	return &api.Listener{
		Name:             name,
		Address:          socketAddress(clusterIP.IP, port.Port),
		TrafficDirection: core.TrafficDirection_OUTBOUND,
		DeprecatedV1: &api.Listener_DeprecatedV1{
			BindToPort: &wrappers.BoolValue{
				Value: false,
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{

					TypedConfig: makeAny(&http.HttpConnectionManager{
						NormalizePath: &wrappers.BoolValue{Value: true},
						MergeSlashes:  true,
						UpgradeConfigs: []*http.HttpConnectionManager_UpgradeConfig{{
							UpgradeType: "websocket",
						}},
						StatPrefix:                prefix,
						PreserveExternalRequestId: true,
						HttpFilters: []*http.HttpFilter{{
							Name: wellknown.Router,
						}},
						RouteSpecifier: &http.HttpConnectionManager_Rds{
							Rds: &http.Rds{
								RouteConfigName: name,
								ConfigSource:    configSource(dynamicClusterName),
							},
						},
					}),
				},
			}},
		}},
	}
}

func gatewayListener(address string, port int, dynamicClusterName string) *api.Listener {
	return &api.Listener{
		Name:             fmt.Sprintf("%s_%s_%d", GatewayListenerPrefix, address, port),
		Address:          socketAddress(address, port),
		TrafficDirection: core.TrafficDirection_INBOUND,
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: makeAny(&http.HttpConnectionManager{
						NormalizePath: &wrappers.BoolValue{Value: true},
						MergeSlashes:  true,
						UpgradeConfigs: []*http.HttpConnectionManager_UpgradeConfig{{
							UpgradeType: "websocket",
						}},
						StatPrefix:                GatewayListenerPrefix,
						UseRemoteAddress:          &wrappers.BoolValue{Value: true},
						PreserveExternalRequestId: true,
						HttpFilters: []*http.HttpFilter{{
							Name: wellknown.Router,
						}},
						RouteSpecifier: &http.HttpConnectionManager_Rds{
							Rds: &http.Rds{
								RouteConfigName: GatewayListenerPrefix,
								ConfigSource:    configSource(dynamicClusterName),
							},
						},
					}),
				},
			}},
		}},
	}
}

func virtualInboundListener(virtualPort int, inboundPorts []k8s.InboundPort) *api.Listener {
	filterChains := []*listener.FilterChain{{
		FilterChainMatch: &listener.FilterChainMatch{
			PrefixRanges: []*core.CidrRange{
				{
					AddressPrefix: "0.0.0.0",
					PrefixLen:     &wrappers.UInt32Value{Value: 0},
				},
			},
		},
		Filters: []*listener.Filter{{
			Name: wellknown.TCPProxy,
			ConfigType: &listener.Filter_TypedConfig{
				TypedConfig: makeAny(&tcp.TcpProxy{
					StatPrefix:       VirtualInboundPrefix,
					ClusterSpecifier: &tcp.TcpProxy_Cluster{Cluster: InboundIPv4ClusterName},
				}),
			},
		}},
	},
	}
	for _, inboundPort := range inboundPorts {
		var filter *listener.Filter
		switch getClusterType(inboundPort.Name) {
		case HTTPCluster:
			filter = &listener.Filter{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: makeAny(&http.HttpConnectionManager{
						GenerateRequestId: &wrappers.BoolValue{Value: true},
						StreamIdleTimeout: &duration.Duration{Seconds: 300},
						StatPrefix:        inboundClusterName(inboundPort.Port),
						HttpFilters: []*http.HttpFilter{{
							Name: wellknown.Router,
						}},
						RouteSpecifier: &http.HttpConnectionManager_RouteConfig{
							RouteConfig: routeInboundConfiguration(
								inboundClusterName(inboundPort.Port),
								inboundClusterName(inboundPort.Port)),
						},
						UpgradeConfigs: []*http.HttpConnectionManager_UpgradeConfig{{
							UpgradeType: "websocket",
						}},
					}),
				},
			}
		case TCPCluster:
			filter = &listener.Filter{
				Name: wellknown.TCPProxy,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: makeAny(&tcp.TcpProxy{
						StatPrefix:       inboundClusterName(inboundPort.Port),
						ClusterSpecifier: &tcp.TcpProxy_Cluster{Cluster: InboundIPv4ClusterName},
					}),
				},
			}
		}
		filterChain := &listener.FilterChain{
			FilterChainMatch: &listener.FilterChainMatch{
				DestinationPort: &wrappers.UInt32Value{Value: uint32(inboundPort.Port)},
			},
			Filters: []*listener.Filter{filter},
		}
		filterChains = append(filterChains, filterChain)
	}
	return &api.Listener{
		Name:             "virtualInbound",
		Address:          socketAddress("0.0.0.0", virtualPort),
		TrafficDirection: core.TrafficDirection_INBOUND,
		UseOriginalDst:   &wrappers.BoolValue{Value: true},
		FilterChains:     filterChains,
	}
}

// socketAddress creates a new TCP core.Address.
func socketAddress(address string, port int) *core.Address {
	return &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.SocketAddress_TCP,
				Address:  address,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}

func makeAny(pb proto.Message) *any.Any {
	any, err := ptypes.MarshalAny(pb)
	if err != nil {
		panic(err.Error())
	}
	return any
}

func ListenerNameFromService(serviceNamespace, serviceName, ip string, port int) string {
	return fmt.Sprintf("%s_%s_%s_%d", serviceNamespace, serviceName, ip, port)
}

func ListenerStatPrefix(serviceNamespace, serviceName string, port int) string {
	return fmt.Sprintf("%s_%s_%d", serviceNamespace, serviceName, port)
}
