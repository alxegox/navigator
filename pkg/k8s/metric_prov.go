package k8s

import (
	"fmt"
	"net/http"

	cache2 "k8s.io/client-go/tools/cache"
)

type counterMetric struct {
	Name  string
	Value uint64
}

func (c *counterMetric) Inc() {
	c.Value++
	fmt.Printf("counter %s: %d\n", c.Name, c.Value)
}

type summaryMetric struct {
	Name  string
	Value float64
	Count uint64
}

func (s *summaryMetric) Observe(v float64) {
	s.Value += v
	s.Count++
	fmt.Printf("summary %s: %f sum %d count\n", s.Name, s.Value, s.Count)
}

type gaugeMetric struct {
	Name  string
	Value float64
}

func (g *gaugeMetric) Set(v float64) {
	g.Value = v
	fmt.Printf("gauge %s: %f \n", g.Name, g.Value)
}

type metricsProvider struct {
	A int
}

func NewMetricsProvider() *metricsProvider {
	mp := &metricsProvider{}
	http.Handle("/", mp)
	go http.ListenAndServe(":8878", http.DefaultServeMux)
	return mp
}

func (m *metricsProvider) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(fmt.Sprintf("len: %d\n", m.A)))
}

func (m *metricsProvider) NewListsMetric(name string) cache2.CounterMetric {
	m.A++
	return &counterMetric{Name: name}
}

func (m *metricsProvider) NewListDurationMetric(name string) cache2.SummaryMetric {
	m.A++
	return &summaryMetric{Name: name}
}

func (m *metricsProvider) NewItemsInListMetric(name string) cache2.SummaryMetric {
	m.A++
	return &summaryMetric{Name: name}
}

func (m *metricsProvider) NewWatchesMetric(name string) cache2.CounterMetric {
	m.A++
	return &counterMetric{Name: name}
}

func (m *metricsProvider) NewShortWatchesMetric(name string) cache2.CounterMetric {
	m.A++
	return &counterMetric{Name: name}
}

func (m *metricsProvider) NewWatchDurationMetric(name string) cache2.SummaryMetric {
	m.A++
	return &summaryMetric{Name: name}
}

func (m *metricsProvider) NewItemsInWatchMetric(name string) cache2.SummaryMetric {
	m.A++
	return &summaryMetric{Name: name}
}

func (m *metricsProvider) NewLastResourceVersionMetric(name string) cache2.GaugeMetric {
	m.A++
	return &gaugeMetric{Name: name}
}
