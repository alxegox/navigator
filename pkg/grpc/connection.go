package grpc

import (
	"io"
	"strconv"

	"github.com/alxegox/navigator/pkg/observability"
	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	errRequestNode      = errors.New("cannot get id and cluster info from request")
	errRequestType      = errors.New("cannot get typeURL from request")
	errGetResources     = errors.New("no resource registered")
	errConvertResources = errors.New("cannot convert resources to any")
	errSendResponce     = errors.New("cannot send responce")
)

type envoyConnection struct {
	logger    logrus.Ext1FieldLogger
	metrics   *observability.Metrics
	resources ResourceGetter
	stream    grpcStream

	// for uniq name and logging
	conID   string
	app     string
	cluster string

	reqCh    chan *v2.DiscoveryRequest
	pushCh   chan PushEvent
	notifyCh chan PushEvent

	// map[typeURL]map[resourceName]version
	typeResourceCache map[string]map[string]int
	resCache          map[string]Resource
	typeCache         map[string]int
	finalError        error
}

func newEnvoyConnecton(logger logrus.Ext1FieldLogger, metrics *observability.Metrics, resources ResourceGetter, stream grpcStream, conID uint64) *envoyConnection {
	return &envoyConnection{
		logger:            logger,
		metrics:           metrics,
		resources:         resources,
		stream:            stream,
		conID:             strconv.FormatUint(conID, 10),
		typeResourceCache: make(map[string]map[string]int),
		typeCache:         make(map[string]int),
		resCache:          make(map[string]Resource),
	}
}

// source: https://github.com/istio/istio/blob/f1a01f44e319624344e5af91fad6b1f810dd71ca/pilot/pkg/proxy/envoy/v2/ads.go#L162
func (e *envoyConnection) receiveRequests() {
	defer close(e.reqCh) // indicates close of the remote side.
	for {
		e.logger.Debug("wait for request")
		req, err := e.stream.Recv()
		if err != nil {
			if isExpectedGRPCError(err) {
				e.logger.WithError(err).Infof("stream terminated")
				return
			}
			e.finalError = err
			e.logger.WithError(err).Errorf("stream terminated with error")
			return
		}
		e.logger.Debug("request accepted")

		select {
		case e.reqCh <- req:
		case <-e.stream.Context().Done():
			e.logger.Info("stream terminated")
			return
		}
	}
}

// receiveNotify is non-bloking resources push processor
// every event saves into buffer before push
// actually it implements endless channel
func (e *envoyConnection) receiveNotify() {

	buf := []PushEvent{}
	for {
		if len(buf) == 0 {
			b, ok := <-e.notifyCh
			if !ok {
				return
			}
			buf = append(buf, b)
		}
		select {
		case b, ok := <-e.notifyCh:
			if !ok {
				return
			}
			buf = append(buf, b)
		case e.pushCh <- buf[0]:
			buf = buf[1:]
		}
	}
}

func (e *envoyConnection) initRequestProcessing(req *v2.DiscoveryRequest) error {
	reqNode := req.GetNode()
	if reqNode == nil {
		e.logger.WithError(errRequestNode).Error("error on getting node info")
		return errRequestNode
	}
	e.app = reqNode.GetId()
	e.cluster = reqNode.GetCluster()

	typeURL := req.GetTypeUrl()
	if typeURL == "" {
		e.logger.WithError(errRequestType).Error("error on getting typeURL")
		return errRequestType
	}
	return nil
}

func (e *envoyConnection) getResource(typeURL string) (Resource, error) {
	logger := e.createResourceLogger(typeURL)
	r, ok := e.resources.Get(e.app, typeURL, e.cluster)
	if !ok {
		logger.WithError(errGetResources).Error("error on getting resources")
		return nil, errGetResources
	}
	return r, nil
}

func (e *envoyConnection) sendResource(typeURL string, last int, resources []proto.Message) error {

	resLog := e.createResourceLogger(typeURL).WithField("count", len(resources))
	any, err := toAny(typeURL, resources)
	if err != nil {
		resLog.WithError(err).Error(errConvertResources.Error())
		return errors.Wrap(err, errConvertResources.Error())
	}

	resp := &v2.DiscoveryResponse{
		VersionInfo: strconv.Itoa(last),
		Resources:   any,
		TypeUrl:     typeURL,
		Nonce:       strconv.Itoa(last),
	}
	err = e.stream.Send(resp)
	if err != nil {
		resLog.WithError(err).Error(errSendResponce.Error())
		return errors.Wrap(err, errSendResponce.Error())
	}

	resLog.Info("resources updated")
	return nil
}

func (e *envoyConnection) updateCache(resource Resource, typeURL string, last int, names []string) {

	resLog := e.createResourceLogger(typeURL)
	e.typeCache[typeURL] = last
	state, ok := e.typeResourceCache[typeURL]
	if !ok {
		state = make(map[string]int, len(names))
	}
	for _, name := range names {
		state[name] = last
	}
	e.typeResourceCache[typeURL] = state
	resLog.Debug("resources updated")
	resource.Register(e.conID, e.notifyCh)
	resLog.Debug("subscribed on updates")
	e.resCache[typeURL] = resource
}

func (e *envoyConnection) unregisterPushChannels() {

	readAll := func(notifyCh <-chan PushEvent) {
		for {
			select {
			case _, ok := <-notifyCh:
				if !ok {
					return
				}
			}
		}
	}

	go readAll(e.notifyCh)
	for _, r := range e.resCache {
		r.Unregister(e.conID)
	}
	close(e.notifyCh)
}

func (e *envoyConnection) deleteMetrics() {
	for typeURL, resources := range e.typeResourceCache {
		for resourceName := range resources {
			labels := prometheus.Labels{
				"type_url":      typeURL,
				"resource_name": resourceName,
				"con_id":        e.conID,
				"app":           e.app,
				"cluster":       e.cluster,
			}
			e.metrics.RejectedResources.Delete(labels)
		}
	}
}

func (e *envoyConnection) isForceUpdateNeeded(typeURL, version string, names []string, last int) bool {

	if v, ok := e.typeCache[typeURL]; !ok || v < last {
		return true
	}

	if version == "" {
		return true
	}

	reqVersion, err := strconv.Atoi(version)
	if err != nil || reqVersion < last {
		for _, name := range names {
			e.metrics.RejectedResources.With(prometheus.Labels{
				"type_url":      typeURL,
				"resource_name": name,
				"con_id":        e.conID,
				"app":           e.app,
				"cluster":       e.cluster,
			}).Inc()
		}
		return true
	}

	// also we need to check whether we provided such resources earlier or not
	for _, name := range names {
		lastNonce, ok := e.typeResourceCache[typeURL][name]
		if !ok || lastNonce < last {
			return true
		}
	}

	return false
}

func (e *envoyConnection) isCacheInvalid(typeURL string, names []string, last int) bool {
	state, ok := e.typeResourceCache[typeURL]
	if !ok {
		return true
	}
	if len(names) == 0 {
		for _, v := range state {
			if v < last {
				return true
			}
		}
	} else {
		for _, name := range names {
			if v, ok := state[name]; !ok || v < last {
				return true
			}
		}
	}
	return false
}

// stream processes a stream of DiscoveryRequests.
func (e *envoyConnection) ProcessStream() error {

	e.reqCh = make(chan *v2.DiscoveryRequest, 1)
	e.pushCh = make(chan PushEvent, 10)
	e.notifyCh = make(chan PushEvent)
	defer e.unregisterPushChannels()
	defer e.deleteMetrics()

	go e.receiveNotify()
	go e.receiveRequests()

	for {
		select {
		case req, ok := <-e.reqCh:
			if !ok {
				// Remote side closed connection.
				return e.finalError
			}
			err := e.initRequestProcessing(req)
			if err != nil {
				return err
			}

			typeURL := req.GetTypeUrl()
			resLog := e.createResourceLogger(typeURL)

			resLog.Debug("get resources")
			r, err := e.getResource(typeURL)
			if err != nil {
				return err
			}
			resLog.Debug("resources received")

			last := r.Nonce()
			version := req.GetVersionInfo()
			names := req.GetResourceNames()

			var resources []proto.Message
			if len(names) == 0 {
				// no resource hints supplied, return the full contents of the resource
				resources, names = r.Contents()
			} else {
				// resource hints supplied, return exactly those
				resources = r.Query(names)
			}

			isForceUpdateNeeded := e.isForceUpdateNeeded(typeURL, version, names, last)
			if !isForceUpdateNeeded {
				isCacheInvalid := e.isCacheInvalid(typeURL, names, last)
				if !isCacheInvalid {
					resLog.Info("resources update no needed")
					continue
				}
			}
			resLog.Debug("start sending resource")

			err = e.sendResource(typeURL, last, resources)
			if err != nil {
				return err
			}

			e.updateCache(r, typeURL, last, names)

		case pushEvent := <-e.pushCh:

			typeURL := pushEvent.ResourceType()
			resLog := e.createResourceLogger(typeURL)
			r, err := e.getResource(typeURL)
			if err != nil {
				return err
			}
			last := pushEvent.Nonce()
			resources, names := r.Contents()

			e.isCacheInvalid(typeURL, names, last)

			err = e.sendResource(typeURL, last, resources)
			if err != nil {
				return err
			}
			resLog.Debug("resource successfully pushed")
		}
	}
}

func (e *envoyConnection) createResourceLogger(typeURL string) logrus.Ext1FieldLogger {
	return e.logger.WithFields(logrus.Fields{
		"app_name":   e.app,
		"cluster_id": e.cluster,
		"type_url":   typeURL,
	})
}

// isExpectedGRPCError checks a gRPC error code and determines whether it is an expected error when
// things are operating normally. This is basically capturing when the client disconnects.
// source: https://github.com/istio/istio/blob/f1a01f44e319624344e5af91fad6b1f810dd71ca/pilot/pkg/proxy/envoy/v2/ads.go#L147
func isExpectedGRPCError(err error) bool {
	if err == io.EOF {
		return true
	}

	s := status.Convert(err)
	if s.Code() == codes.Canceled || s.Code() == codes.DeadlineExceeded {
		return true
	}
	if s.Code() == codes.Unavailable && s.Message() == "client disconnected" {
		return true
	}
	return false
}
