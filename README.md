<!-- LoadRunner Logo -->
<p align="center">
  <img src="https://upload.wikimedia.org/wikipedia/commons/2/2e/Micro_Focus_LoadRunner_Logo.png" alt="LoadRunner Logo" width="200"/>
</p>

# sumdat2otel

**sumdat2otel** is a post-processing/export tool designed to send LoadRunner test results to an OpenTelemetry endpoint after a LoadRunner test run.

After your LoadRunner scenario completes, this tool parses the results (sum_dat.ini and graph_X.dat files) and exports the metrics to your observability backend (OTLP or console), enabling integration with modern observability platforms.

---

> **Detailed Documentation:**
> For in-depth usage, logic, workflow, and file format examples, see [src/README.md](src/README.md).

---

## Features
- Recursively processes all LoadRunner results folders in a given parent directory
- For each results folder, parses the `sum_data/sum_dat.ini` and all `graph_X.dat` files
- Extracts graph type, measurement names, values, counts, and timestamps
- Handles counter metrics (count == -1) and calculates per-second rates
- Exports both value and count as separate metrics
- Preserves all relevant labels (graph type, measurement, results folder, etc.)
- Special handling for load generator monitoring graphs (labels for loadgenerator and metric type)
- Configurable logging and exporter (console or OTLP)

## Metric Naming Conventions
- For each graph/measurement, the following metrics may be exported:
  - `<graph_type>_value`: The main value (gauge/histogram)
  - `<graph_type>_count`: The count of events/samples in the interval (for rate metrics)
  - `<graph_type>_counter`: The raw counter value (for counter metrics, count == -1)
  - `<graph_type>_rate`: The calculated rate (delta per second) for counter metrics
- For load generator monitoring, metric names are prefixed with `lg_` and labels include `loadgenerator` and `metric_type`.

## How to Interpret the Metrics
- **_value**: The main value for the metric (e.g., CPU usage, response time, etc.)
- **_count**: The number of events/samples in the interval (e.g., requests per second)
- **_counter**: The cumulative value for counter metrics (e.g., total bytes sent)
- **_rate**: The per-second rate calculated from the counter (e.g., bytes per second)
- **Labels**: Each metric includes labels for graph type, measurement name, results folder, project, test, and (for lg_monitoring) loadgenerator and metric type.

## Quick Start: Build, Test, and Docker

All commands below are run from the project root (where the Makefile is located).

### Build the Go Binary

```sh
make build
```
This builds the `lr2otelmetric` binary in the project root.

### Run Unit Tests

```sh
make test
```
This runs all Go tests in the `src/` directory.

### Build Docker Image

By default, builds for `linux/amd64`:
```sh
make docker
```

To build for ARM64 (e.g., Apple Silicon, AWS Graviton):
```sh
make docker PLATFORM=linux/arm64
```

This produces a Docker image named `lr2otelmetric`.

### Run the Utility

After building (either natively or in Docker):

```sh
./lr2otelmetric <project_name> <test_name> /path/to/results
```

Or with Docker:
```sh
docker run --rm -v /path/to/data:/data lr2otelmetric <project_name> <test_name> /data
```

## Usage

```sh
./lr2otelmetric <project_name> <test_name> <parent_results_folder>
```
- `<project_name>`: Name of the project (added as a label to all metrics)
- `<test_name>`: Name of the test (added as a label to all metrics)
- `<parent_results_folder>`: Path to the folder containing multiple LoadRunner results folders. Each subfolder should contain a `sum_data` directory.

## Environment Variables

- `CONSOLEMODE` (default: false):
  - If set to `true`, metrics are printed to the console (stdout).
  - If not set or set to any other value, metrics are exported to an OTLP gRPC endpoint.
- `OTEL_EXPORTER_OTLP_ENDPOINT` (default: `localhost:4317`):
  - The OTLP gRPC endpoint to which metrics are sent (when not in console mode).
- `project_name` and `test_name` are required CLI parameters and are always included as labels in all metrics.

## Example

### Export metrics to OTLP endpoint (default)
```sh
export OTEL_EXPORTER_OTLP_ENDPOINT="otel-collector:4317"
./lr2otelmetric MyProject MyTest ResultsParentFolder
```

### Print metrics to console
```sh
export CONSOLEMODE=true
./lr2otelmetric MyProject MyTest ResultsParentFolder
```

### Using OTLP Exporter (Environment Variables)

You can configure the OTLP exporter by setting environment variables. For example, to export metrics to an OTLP gRPC endpoint:

#### CLI Example

```sh
export OTEL_EXPORTER_OTLP_ENDPOINT="otel-collector:4317"
export OTEL_EXPORTER_OTLP_INSECURE=true
export OTEL_METRIC_EXPORTER=otlp
export LOG_LEVEL=debug
export TIMEZONE=Europe/Paris
./lr2otelmetric MyProject MyTest /path/to/results
```

#### Docker Example

```sh
docker run --rm \
  -v /path/to/data:/data \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317 \
  -e OTEL_EXPORTER_OTLP_INSECURE=true \
  -e OTEL_METRIC_EXPORTER=otlp \
  -e LOG_LEVEL=debug \
  -e TIMEZONE=Europe/Paris \
  lr2otelmetric MyProject MyTest /data
```

Replace `/path/to/data` with the directory containing your LoadRunner logs or results. Adjust the OTLP endpoint and other variables as needed for your environment. Replace `MyProject` and `MyTest` with your desired project and test names.

## Output
- Metrics are exported as OpenTelemetry metrics (see naming conventions above)
- Each metric includes relevant labels for filtering and analysis, including `project` and `test`
- The tool prints which results folder is being processed

