package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alxegox/navigator/tests/e2e/tests"
)

var (
	defaultComponentList []string
	listenAddr           = ":80"
	defaultProbePort     = "8999"
	requestTimeout       = 1000 * time.Millisecond
	dnsPostfix           = "svc.cluster.local"
	defaultProbesCount   = 10
	parallelProbesCount  = 10
	defaultAppList       []string
	timeoutRetries       = 3
)

func main() {
	if len(os.Args) > 1 {
		listenAddr = os.Args[1]
	}

	appListEnv := os.Getenv("APP_LIST")
	if appListEnv != "" {
		defaultAppList = strings.Split(appListEnv, " ")
	}

	componentListEnv := os.Getenv("COMPONENT_LIST")
	if componentListEnv != "" {
		defaultComponentList = strings.Split(componentListEnv, " ")
	}

	http.HandleFunc("/", handleAggregatedStats)

	//warmup
	//_, _ = getAggregatedStats(defaultProbesCount)
	err := http.ListenAndServe(listenAddr, nil)
	if err != nil {
		panic(err)
	}
}

func handleAggregatedStats(w http.ResponseWriter, r *http.Request) {
	appList := r.URL.Query()["appList"]
	if len(appList) == 0 {
		appList = defaultAppList
	}

	componentList := r.URL.Query()["componentList"]
	if len(componentList) == 0 {
		componentList = defaultComponentList
	}

	log.Printf("quaey: %v\ncomponentList:%v\n\n", r.URL.Query()["componentList"], componentList)

	count := defaultProbesCount
	if len(r.URL.Query()["count"]) > 0 {
		if tCount, _ := strconv.Atoi(r.URL.Query()["count"][0]); tCount > 0 {
			count = tCount
		}
	}

	probePort := defaultProbePort
	if len(r.URL.Query()["probePort"]) > 0 {
		probePort = r.URL.Query()["probePort"][0]
	}

	uri := "/"
	if len(r.URL.Query()["url"]) > 0 {
		uri = r.URL.Query()["url"][0]
	}

	componentsProtos := map[string]string{}
	for _, componentProto := range r.URL.Query()["componentsProtos"] {
		parts := strings.Split(componentProto, ":")
		if len(parts) == 2 {
			componentsProtos[parts[0]] = parts[1]
		}
	}

	log.Println(componentsProtos)

	response, err := getAggregatedStats(appList, componentList, probePort, uri, count, componentsProtos)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fmt.Sprintf("Failed to get stat: %s", err.Error())))
		return
	}

	w.Header().Add("Content-Type", "application/json")
	_, _ = w.Write(response)
}

func getAggregatedStats(appList, componentList []string, probePort, uri string, probesCount int, componentsProtos map[string]string) ([]byte, error) {
	var wg sync.WaitGroup

	var mu sync.Mutex
	stats := map[string]*tests.ComponentStat{}

	for _, appName := range appList {
		for _, componentName := range componentList {
			if appName == "" || componentName == "" {
				continue
			}
			wg.Add(1)
			go func(appName, componentName string, probesCount int) {

				stat := getComponentStats(appName, componentName, probePort, uri, probesCount, componentsProtos)

				mu.Lock()
				stats[stat.ComponentKey.String()] = stat
				mu.Unlock()

				wg.Done()
			}(appName, componentName, probesCount)
		}
	}

	wg.Wait()

	return json.Marshal(stats)
}

func getComponentStats(AppName, ComponentName, probePort, uri string, count int, componentsProtos map[string]string) *tests.ComponentStat {
	log.Printf("getComponentStats %v %v %v \n", AppName, ComponentName, count)

	var wg sync.WaitGroup
	wg.Add(count)

	stat := &tests.ComponentStat{ComponentKey: tests.ComponentKey{AppName: AppName, ComponentName: ComponentName}, TotalProbes: count}
	semaphore := make(chan struct{}, parallelProbesCount)
	var netClient = &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout: 30 * time.Second,
			}).DialContext,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			url := fmt.Sprintf("http://%s.%s.%s:%s%s", ComponentName, AppName, dnsPostfix, probePort, uri)

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				stat.AddFail(err)
				return
			}

			if componentsProtos[ComponentName] != "http" {
				req.Header.Set("Connection", "close")
			}

			start := time.Now()
			var response *http.Response
			for r := timeoutRetries; r > 0; r-- {
				response, err = netClient.Do(req)
				if err == nil {
					break
				}

				log.Printf("%+v\n", err)
			}
			if err != nil {
				stat.AddFail(err)
				return
			}

			buf, err := ioutil.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil {
				stat.AddFail(err)
				return
			}

			if response.StatusCode != http.StatusOK {
				stat.AddFail(fmt.Errorf("status: %d,   %s", response.StatusCode, buf))
				return
			}

			stat.AddResponse(string(buf), time.Since(start))
		}()
	}

	wg.Wait()

	return stat
}
