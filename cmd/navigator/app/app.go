package app

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/alxegox/navigator/pkg/apis/generated/clientset/versioned"
	"github.com/alxegox/navigator/pkg/grpc"
	"github.com/alxegox/navigator/pkg/k8s"
	"github.com/alxegox/navigator/pkg/observability"
	"github.com/alxegox/navigator/pkg/xds"
	"github.com/alxegox/navigator/pkg/xds/resources"
)

type Config struct {
	KubeConfigs              []string
	ServingAddress           string
	GRPCPort                 string
	HTTPPort                 string
	Loglevel                 string
	LogJSONFormatter         bool
	EnableProfiling          bool
	EnableSidecarHealthCheck bool
	SidecarHealthCheckPath   string
	SidecarHealthCheckPort   int
	SidecarInboundPort       int
	LocalityEnabled          bool
}

func Run(config Config) {
	registry := prometheus.NewRegistry()
	registry.Register(prometheus.NewGoCollector())
	registry.Register(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	metrics := observability.NewMetrics(registry)

	metrics.GRPCRunning.Set(0)

	logger, err := getLogger(config.Loglevel, config.LogJSONFormatter)
	if err != nil {
		logrus.WithError(err).Fatal("cannot create logger")
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	setSigintHandler(logger, cancelFunc)

	k8sCache := k8s.NewCache(logger, metrics)
	canary := k8s.NewCanary(metrics)
	nexusCache := k8s.NewNexusCache()
	ingressCache := k8s.NewIngressCache()
	gatewayCache := k8s.NewGatewayCache()

	opts := getResourcesOpts(config)
	xdsCache := xds.NewCache(k8sCache, nexusCache, ingressCache, gatewayCache, logger, opts)

	grpcServer := grpc.NewAPI(logger, metrics, xdsCache)
	if len(config.KubeConfigs) == 0 {
		logger.Fatalf("At least 1 kubeconfig must be specified")
	}

	mux := http.NewServeMux()
	observability.RegisterMetrics(mux, registry)
	observability.RegisterHealthCheck(mux)
	if config.EnableProfiling {
		observability.RegisterProfile(mux)
	}
	mux.HandleFunc("/debug/app/k8s", xdsCache.GetAppsK8sState)
	mux.HandleFunc("/debug/app/envoy", xdsCache.GetAppsEnvoyState)

	httpServer := &http.Server{
		Addr:    net.JoinHostPort(config.ServingAddress, config.HTTPPort),
		Handler: mux,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			logger.Errorf("Failed to serve metrics: %v", err)
		}
	}()

	for _, kubeConfig := range config.KubeConfigs {
		cs, err := getK8sClient(kubeConfig)
		if err != nil {
			logger.WithError(err).Fatalf("Failed to create k8s client with config %q", kubeConfig)
		}
		extCs, err := getExtK8sClient(kubeConfig)
		if err != nil {
			logger.WithError(err).Fatalf("Failed to create k8s cdr client with config %q", kubeConfig)
		}

		k8sWatcher := k8s.NewWatcher(
			logger,
			metrics,
			cs,
			extCs,
			k8sCache,
			canary,
			xdsCache,
			nexusCache,
			ingressCache,
			gatewayCache,
			kubeConfig,
		)
		k8sWatcher.Start(ctx)
	}

	fullAddr := net.JoinHostPort(config.ServingAddress, config.GRPCPort)
	l, err := net.Listen("tcp", fullAddr)
	if err != nil {
		logger.WithError(err).Fatalf("Failed to listen %q", fullAddr)
	}

	go func() {
		err = grpcServer.Serve(l)
		if err != nil {
			logger.Errorf("Failed to serve GRPC: %v", err)
		}
	}()
	metrics.GRPCRunning.Set(1)

	<-ctx.Done()
	grpcServer.Stop()
	metrics.GRPCRunning.Set(0)

	httpCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(httpCtx)
}

func getLogger(level string, json bool) (*logrus.Logger, error) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, err
	}
	logger := logrus.StandardLogger()
	logger.SetLevel(lvl)
	if json {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}
	return logger, nil
}

func getK8sClient(kubeConfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	return client, err
}

func getExtK8sClient(kubeConfig string) (*versioned.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}

	client, err := versioned.NewForConfig(config)
	return client, err
}

func setSigintHandler(logger logrus.FieldLogger, cancelFunc context.CancelFunc) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)

	go func() {
		<-sigs
		logger.Info("SIGINT signal caught, stopping...")
		cancelFunc()
	}()
}

func getResourcesOpts(config Config) []resources.FuncOpt {
	opts := []resources.FuncOpt{}
	if dynamicClusterName, ok := os.LookupEnv("NAVIGATOR_DYNAMIC_CLUSTER"); ok {
		opts = append(opts, resources.WithClusterName(dynamicClusterName))
	}
	if config.EnableSidecarHealthCheck {
		opts = append(opts, resources.WithHealthCheck(config.SidecarHealthCheckPath, config.SidecarHealthCheckPort))
	}
	if config.LocalityEnabled {
		opts = append(opts, resources.WithLocalityEnabled())
	}
	opts = append(opts, resources.WithInboundPort(config.SidecarInboundPort))
	return opts
}
