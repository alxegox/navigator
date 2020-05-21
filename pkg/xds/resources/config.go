package resources

import "strings"

const (
	DefaultDynamicClusterName = "navigator"

	healthCheckNamePrefix = "~~"
)

type FuncOpt func(resource interface{})

type DynamicConfiger interface {
	SetClusterName(name string)
}

type DynamicConfig struct {
	DynamicClusterName string
}

func (c *DynamicConfig) SetClusterName(name string) {
	c.DynamicClusterName = name
}

func WithClusterName(name string) FuncOpt {
	return func(resource interface{}) {
		r, ok := resource.(DynamicConfiger)
		if !ok {
			return
		}
		r.SetClusterName(name)
	}
}

type LocalityEnabler interface {
	SetLocalityEnabled(bool)
}

func WithLocalityEnabled() FuncOpt {
	return func(resource interface{}) {
		r, ok := resource.(LocalityEnabler)
		if !ok {
			return
		}
		r.SetLocalityEnabled(true)
	}
}

type InboundPortConfiger interface {
	SetInboundPort(int)
}

type InboundPortConfig struct {
	InboundPort int
}

func (ipc *InboundPortConfig) SetInboundPort(port int) {
	ipc.InboundPort = port
}

func WithInboundPort(port int) FuncOpt {
	return func(resource interface{}) {
		r, ok := resource.(InboundPortConfiger)
		if !ok {
			return
		}
		r.SetInboundPort(port)
	}
}

type HealthCheckConfiger interface {
	SetHealthCheck(path string, port int)
}

type HealthCheckConfig struct {
	EnableHealthCheck bool
	HealthCheckName   string
	HealthCheckPath   string
	HealthCheckPort   int
}

func (c *HealthCheckConfig) SetHealthCheck(path string, port int) {

	c.EnableHealthCheck = true
	c.HealthCheckPort = port

	if strings.HasPrefix(path, "/") {
		c.HealthCheckPath = path
		c.HealthCheckName = healthCheckNamePrefix + strings.TrimPrefix(path, "/")
	} else {
		c.HealthCheckPath = "/" + path
		c.HealthCheckName = healthCheckNamePrefix + path
	}
}

func WithHealthCheck(path string, port int) FuncOpt {
	return func(resource interface{}) {
		r, ok := resource.(HealthCheckConfiger)
		if !ok {
			return
		}
		r.SetHealthCheck(path, port)
	}
}

type AppNameConfiger interface {
	SetAppName(appName string)
}
type AppNameConfig struct {
	AppName string
}

func (c *AppNameConfig) SetAppName(appName string) {
	c.AppName = appName
}

func WithAppName(appName string) FuncOpt {
	return func(resource interface{}) {
		r, ok := resource.(AppNameConfiger)
		if !ok {
			return
		}
		r.SetAppName(appName)
	}
}

func SetOpts(resource interface{}, opts ...FuncOpt) {
	for _, opt := range opts {
		opt(resource)
	}
}
