# lr2otelmetric Documentation

## Overview

**lr2otelmetric** is a utility for post-processing LoadRunner test results and exporting them as OpenTelemetry metrics. It enables integration of LoadRunner performance data with modern observability platforms by converting LoadRunner's proprietary results into standard OTLP metrics.

After a LoadRunner scenario completes, this tool parses the results (notably `sum_dat.ini` and `graph_X.dat` files) and exports the metrics to your observability backend (OTLP or console).

## How to Launch

### Command Line

```sh
./lr2otelmetric <project_name> <test_name> <parent_results_folder>
```
- `<project_name>`: Name of the project (added as a label to all metrics)
- `<test_name>`: Name of the test (added as a label to all metrics)
- `<parent_results_folder>`: Path to the folder containing multiple LoadRunner results folders. Each subfolder should contain a `sum_data` directory.

### Docker

```sh
docker run --rm -v /path/to/data:/data lr2otelmetric <project_name> <test_name> /data
```
- Replace `/path/to/data` with the directory containing your LoadRunner results.
- Replace `<project_name>` and `<test_name>` with your desired values.

### Environment Variables

- `CONSOLEMODE` (default: false):
  - If set to `true`, metrics are printed to the console (stdout).
  - If not set or set to any other value, metrics are exported to an OTLP gRPC endpoint.
- `OTEL_EXPORTER_OTLP_ENDPOINT` (default: `localhost:4317`):
  - The OTLP gRPC endpoint to which metrics are sent (when not in console mode).
- `OTEL_EXPORTER_OTLP_INSECURE`, `OTEL_METRIC_EXPORTER`, `LOG_LEVEL`, `TIMEZONE`:
  - Additional OpenTelemetry and logging configuration (see main README for details).
- `project_name` and `test_name` are required CLI parameters and are always included as labels in all metrics.

## Logic and Workflow

1. **Recursive Folder Processing**
   - The tool scans the given parent directory for all subfolders containing a `sum_data` directory.
   - Each `sum_data` folder is processed independently.

2. **Parsing sum_dat.ini**
   - For each `sum_data` folder, the tool parses `sum_dat.ini` to discover available graphs and their measurements.
   - Each graph section defines a `GraphType` and a set of measurements (name and ID).

   **Example `sum_dat.ini`:**
   ```ini
   [graph_0]
   GraphType=es_tr_runtime_vusers
   Measurement_num=1
   Measurement_0=Running,1

   [graph_1]
   GraphType=es_tr_lg_monitoring
   Measurement_num=2
   Measurement_0=host1 - CPU Usage,2
   Measurement_1=host2 - Memory Usage,3
   ```
   - Each `[graph_X]` section describes a graph.
   - `GraphType` is the metric type/category.
   - Each `Measurement_N` gives a measurement name and its ID (used in the .dat file).

3. **Parsing graph_X.dat Files**
   - For each graph, the corresponding `graph_X.dat` file is parsed line by line.
   - Each line contains: measurement ID, timestamp, value, count, min, max, sum.
   - The tool maps measurement IDs to names using the parsed `sum_dat.ini`.

   **Example `graph_0.dat`:**
   ```
   1 1748439937 23.372772 1 23.372772 23.372772 23.372772
   1 1748439997 25.000000 1 25.000000 25.000000 25.000000
   ```
   - `1`: Measurement ID (matches the ID in sum_dat.ini)
   - `1748439937`: Unix timestamp (seconds)
   - `23.372772`: Value for this interval
   - `1`: Count (number of samples/events, or -1 for counter)
   - `23.372772 23.372772 23.372772`: Min, Max, Sum for the interval

4. **Metric Extraction and Semantics**
   - **_value**: The main value for the metric (e.g., CPU usage, response time, etc.)
   - **_count**: The number of events/samples in the interval (e.g., requests per second)
   - **_counter**: The cumulative value for counter metrics (e.g., total bytes sent, when count == -1)
   - **_rate**: The per-second rate calculated from the counter (e.g., bytes per second)
   - For load generator monitoring, metric names are prefixed with `lg_` and labels include `loadgenerator` and `metric_type`.
   - **All metrics include the `project` and `test` labels as provided on the command line.**

5. **OpenTelemetry Export**
   - Metrics are exported as OpenTelemetry metrics, either to the console or to an OTLP endpoint, depending on environment variables.
   - Each metric includes labels for graph type, measurement name, results folder, project, test, and (for lg_monitoring) loadgenerator and metric type.

6. **Logging and Output**
   - The tool prints which results folder is being processed and any warnings or errors encountered.
   - Logging verbosity and format can be controlled via environment variables.

## Example Usage

Export metrics to OTLP endpoint (default):
```sh
export OTEL_EXPORTER_OTLP_ENDPOINT="otel-collector:4317"
./lr2otelmetric MyProject MyTest ResultsParentFolder
```

Print metrics to console:
```sh
export CONSOLEMODE=true
./lr2otelmetric MyProject MyTest ResultsParentFolder
```

## Output
- Metrics are exported as OpenTelemetry metrics (see naming conventions above)
- Each metric includes relevant labels for filtering and analysis, including `project` and `test`
- The tool prints which results folder is being processed

## Further Information
- For build, test, and Docker image instructions, see the main README in the project root.
- For details on metric naming conventions and advanced configuration, see the main README or the code comments. 