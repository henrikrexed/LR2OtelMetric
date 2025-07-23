package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type Measurement struct {
	Name string
	ID   string
}

type Graph struct {
	GraphType    string
	Measurements []Measurement
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: lr2otelmetric <project_name> <test_name> <parent_results_folder>")
		os.Exit(1)
	}
	projectName := os.Args[1]
	testName := os.Args[2]
	parentDir := os.Args[3]

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		fmt.Printf("Failed to read parent directory: %v\n", err)
		os.Exit(1)
	}

	// OTEL exporter selection (shared for all results)
	ctx := context.Background()
	var reader *sdkmetric.ManualReader
	var provider *sdkmetric.MeterProvider
	var meter metric.Meter

	consoleMode := strings.ToLower(os.Getenv("CONSOLEMODE")) == "true"
	if consoleMode {
		exp, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
		if err != nil {
			fmt.Printf("failed to initialize stdout exporter: %v\n", err)
			os.Exit(1)
		}
		reader = sdkmetric.NewManualReader()
		provider = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
		)
		_ = exp
		fmt.Println("Using console exporter for metrics.")
	} else {
		otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if otlpEndpoint == "" {
			otlpEndpoint = "localhost:4317"
		}
		_, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(otlpEndpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			fmt.Printf("failed to initialize OTLP exporter: %v\n", err)
			os.Exit(1)
		}
		reader = sdkmetric.NewManualReader()
		provider = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
		)
		fmt.Printf("Using OTLP exporter for metrics. Endpoint: %s\n", otlpEndpoint)
	}
	meter = provider.Meter("sumdat-metrics")

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		resultsFolder := filepath.Join(parentDir, entry.Name())
		// Recursively search for all sum_data folders under resultsFolder
		sumDataDirs := []string{}
		filepath.WalkDir(resultsFolder, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip on error
			}
			if d.IsDir() && strings.EqualFold(d.Name(), "sum_data") {
				sumDataDirs = append(sumDataDirs, path)
			}
			return nil
		})
		for _, sumDataDir := range sumDataDirs {
			sumDatIni := filepath.Join(sumDataDir, "sum_dat.ini")
			if _, err := os.Stat(sumDatIni); err != nil {
				continue // skip if no sum_dat.ini
			}
			fmt.Printf("Processing sum_data folder: %s\n", sumDataDir)
			graphs, err := parseSumDatIni(sumDatIni)
			if err != nil {
				fmt.Printf("Failed to parse sum_dat.ini in %s: %v\n", sumDataDir, err)
				continue
			}
			// For counter rate calculation: map[metricName+id]lastValue
			lastCounter := make(map[string]struct {
				ts    int64
				value float64
			})
			for graphName, graph := range graphs {
				datFile := filepath.Join(sumDataDir, fmt.Sprintf("%s.dat", graphName))
				if _, err := os.Stat(datFile); err != nil {
					fmt.Printf("Warning: %s not found, skipping\n", datFile)
					continue
				}
				idToName := map[string]string{}
				for _, m := range graph.Measurements {
					idToName[m.ID] = m.Name
				}
				f, err := os.Open(datFile)
				if err != nil {
					fmt.Printf("Error opening %s: %v\n", datFile, err)
					continue
				}
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := scanner.Text()
					parts := strings.Fields(line)
					if len(parts) < 7 {
						continue
					}
					id, tsStr, valueStr, countStr := parts[0], parts[1], parts[2], parts[3]
					minStr, maxStr, sumStr := parts[4], parts[5], parts[6]
					name, ok := idToName[id]
					if !ok {
						continue
					}
					ts, _ := strconv.ParseInt(tsStr, 10, 64)
					value, _ := strconv.ParseFloat(valueStr, 64)
					count, _ := strconv.Atoi(countStr)
					minv, _ := strconv.ParseFloat(minStr, 64)
					maxv, _ := strconv.ParseFloat(maxStr, 64)
					sumv, _ := strconv.ParseFloat(sumStr, 64)
					attrs := []attribute.KeyValue{
						attribute.String("project", projectName),
						attribute.String("test", testName),
						attribute.String("graph_type", graph.GraphType),
						attribute.String("measurement", name),
						attribute.String("results_folder", entry.Name()),
					}
					if graph.GraphType == "es_tr_lg_monitoring" {
						parts := strings.SplitN(name, " - ", 2)
						if len(parts) == 2 {
							attrs = []attribute.KeyValue{
								attribute.String("project", projectName),
								attribute.String("test", testName),
								attribute.String("graph_type", graph.GraphType),
								attribute.String("loadgenerator", strings.TrimSpace(parts[0])),
								attribute.String("metric_type", strings.TrimSpace(parts[1])),
								attribute.String("results_folder", entry.Name()),
							}
						}
					}
					if strings.Contains(graph.GraphType, "error") || strings.Contains(graph.GraphType, "response_time") || strings.Contains(graph.GraphType, "transaction") {
						attrs = append(attrs, attribute.String("transaction", name))
					}
					metricName := graph.GraphType
					if graph.GraphType == "es_tr_lg_monitoring" && len(attrs) > 2 {
						metricName = fmt.Sprintf("lg_%s", attrs[4].Value.AsString())
					}
					// Export value as histogram (main value)
					hist, err := meter.Float64Histogram(metricName + "_value")
					if err == nil {
						hist.Record(context.Background(), value, metric.WithAttributes(attrs...))
					}
					// Export count as a separate metric (e.g., requests_per_second)
					if count > 0 {
						cnt, err := meter.Int64Histogram(metricName + "_count")
						if err == nil {
							cnt.Record(context.Background(), int64(count), metric.WithAttributes(attrs...))
						}
					}
					// If count == -1, treat as counter and calculate rate
					if count == -1 {
						counterName := metricName + "_counter"
						ctr, err := meter.Float64Histogram(counterName)
						if err == nil {
							ctr.Record(context.Background(), value, metric.WithAttributes(attrs...))
						}
						// Calculate rate if previous value exists
						key := metricName + ":" + id
						if prev, ok := lastCounter[key]; ok && ts > prev.ts {
							delta := value - prev.value
							dt := float64(ts - prev.ts)
							if dt > 0 && delta >= 0 {
								rate, err := meter.Float64Histogram(metricName + "_rate")
								if err == nil {
									rate.Record(context.Background(), delta/dt, metric.WithAttributes(attrs...))
								}
							}
						}
						lastCounter[key] = struct {
							ts    int64
							value float64
						}{ts, value}
					}
					_ = minv
					_ = maxv
					_ = sumv
				}
				f.Close()
			}
			fmt.Printf("Metrics export completed for %s.\n", sumDataDir)
		}
	}
}

// parseSumDatIni parses sum_dat.ini and returns a map of graph name to Graph struct
func parseSumDatIni(path string) (map[string]Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	graphs := map[string]Graph{}
	var currentGraph string
	var graphType string
	var measurements []Measurement
	reGraph := regexp.MustCompile(`^\[graph_(\d+)\]`)
	reType := regexp.MustCompile(`^GraphType=(.*)`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if m := reGraph.FindStringSubmatch(line); m != nil {
			if currentGraph != "" {
				graphs[currentGraph] = Graph{GraphType: graphType, Measurements: measurements}
			}
			currentGraph = fmt.Sprintf("graph_%s", m[1])
			graphType = ""
			measurements = nil
			continue
		}
		if m := reType.FindStringSubmatch(line); m != nil {
			graphType = m[1]
			continue
		}
		if strings.HasPrefix(line, "Measurement_") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				vals := strings.SplitN(parts[1], ",", 2)
				if len(vals) == 2 {
					measurements = append(measurements, Measurement{
						Name: strings.TrimSpace(vals[0]),
						ID:   strings.TrimSpace(vals[1]),
					})
				}
			}
		}
	}
	if currentGraph != "" {
		graphs[currentGraph] = Graph{GraphType: graphType, Measurements: measurements}
	}
	return graphs, nil
}
