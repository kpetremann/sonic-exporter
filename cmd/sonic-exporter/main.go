package main

import (
	"net/http"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	nodecollector "github.com/prometheus/node_exporter/collector"
	frrcollector "github.com/tynany/frr_exporter/collector"
	"github.com/vinted/sonic-exporter/internal/collector"
)

func main() {
	var (
		webConfig   = webflag.AddFlags(kingpin.CommandLine, ":9101")
		metricsPath = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	)

	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.Parse()

	logger := promslog.New(promslogConfig)

	// SONiC collectors
	interfaceCollector := collector.NewInterfaceCollector(logger)
	hwCollector := collector.NewHwCollector(logger)
	crmCollector := collector.NewCrmCollector(logger)
	queueCollector := collector.NewQueueCollector(logger)
	prometheus.MustRegister(interfaceCollector)
	prometheus.MustRegister(hwCollector)
	prometheus.MustRegister(crmCollector)
	prometheus.MustRegister(queueCollector)

	// Node exporter collectors
	nodeCollector, err := nodecollector.NewNodeCollector(logger,
		"loadavg",
		"cpu",
		"diskstats",
		"filesystem",
		"meminfo",
		"time",
		"stat",
	)
	if err != nil {
		logger.Error("Failed to create node collector", "error", err)
		os.Exit(1)
	}
	prometheus.MustRegister(nodeCollector)

	// FRR exporter
	frrExporter, err := frrcollector.NewExporter(logger)
	if err != nil {
		logger.Error("Failed to create FRR exporter", "error", err)
		os.Exit(1)
	}

	bgpL2VPNCollector, err := frrcollector.NewBGPL2VPNCollector(logger)
	if err != nil {
		logger.Error("Failed to create BGP L2VPN collector", "error", err)
		os.Exit(1)
	}
	frrExporter.Collectors["bgp_l2vpn"] = bgpL2VPNCollector

	bgpCollector, err := frrcollector.NewBGPCollector(logger)
	if err != nil {
		logger.Error("Failed to create BGP collector", "error", err)
		os.Exit(1)
	}
	frrExporter.Collectors["bgp"] = bgpCollector

	bgp6Collector, err := frrcollector.NewBGP6Collector(logger)
	if err != nil {
		logger.Error("Failed to create BGP6 collector", "error", err)
		os.Exit(1)
	}
	frrExporter.Collectors["bgp6"] = bgp6Collector

	statusCollector, err := frrcollector.NewStatusCollector(logger)
	if err != nil {
		logger.Error("Failed to create FRR status collector", "error", err)
		os.Exit(1)
	}
	frrExporter.Collectors["status"] = statusCollector

	routeCollector, err := frrcollector.NewRouteCollector(logger)
	if err != nil {
		logger.Error("Failed to create route collector", "error", err)
		os.Exit(1)
	}
	frrExporter.Collectors["route"] = routeCollector

	prometheus.MustRegister(frrExporter)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
             <head><title>Sonic Exporter</title></head>
             <body>
             <h1>Sonic Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
		if err != nil {
			logger.Error("Error writing response", "error", err)
		}
	})
	srv := &http.Server{}
	if err := web.ListenAndServe(srv, webConfig, logger); err != nil {
		logger.Error("Error starting HTTP server", "error", err)
		os.Exit(1)
	}
}
