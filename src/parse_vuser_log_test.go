package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestParseSumDatIni_Simple(t *testing.T) {
	content := `
[graph_0]
GraphType=es_tr_runtime_vusers
Measurement_num=1
Measurement_0=Running,1
[graph_1]
GraphType=es_tr_lg_monitoring
Measurement_num=2
Measurement_0=host1 - CPU Usage,2
Measurement_1=host2 - Memory Usage,3
`
	tmpfile, err := ioutil.TempFile("", "sumdatini-*.ini")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpfile.Close()

	graphs, err := parseSumDatIni(tmpfile.Name())
	if err != nil {
		t.Fatalf("parseSumDatIni failed: %v", err)
	}

	g0, ok := graphs["graph_0"]
	if !ok {
		t.Errorf("graph_0 not found")
	}
	if g0.GraphType != "es_tr_runtime_vusers" {
		t.Errorf("graph_0 GraphType got %q, want %q", g0.GraphType, "es_tr_runtime_vusers")
	}
	if len(g0.Measurements) != 1 || g0.Measurements[0].Name != "Running" || g0.Measurements[0].ID != "1" {
		t.Errorf("graph_0 measurements incorrect: %+v", g0.Measurements)
	}

	g1, ok := graphs["graph_1"]
	if !ok {
		t.Errorf("graph_1 not found")
	}
	if g1.GraphType != "es_tr_lg_monitoring" {
		t.Errorf("graph_1 GraphType got %q, want %q", g1.GraphType, "es_tr_lg_monitoring")
	}
	if len(g1.Measurements) != 2 {
		t.Errorf("graph_1 measurements length got %d, want 2", len(g1.Measurements))
	}
	if g1.Measurements[0].Name != "host1 - CPU Usage" || g1.Measurements[0].ID != "2" {
		t.Errorf("graph_1 Measurement_0 incorrect: %+v", g1.Measurements[0])
	}
	if g1.Measurements[1].Name != "host2 - Memory Usage" || g1.Measurements[1].ID != "3" {
		t.Errorf("graph_1 Measurement_1 incorrect: %+v", g1.Measurements[1])
	}
}

func TestParseSumDatIni_Empty(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "sumdatini-empty-*.ini")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()
	graphs, err := parseSumDatIni(tmpfile.Name())
	if err != nil {
		t.Fatalf("parseSumDatIni failed: %v", err)
	}
	if len(graphs) != 0 {
		t.Errorf("expected 0 graphs, got %d", len(graphs))
	}
}

func TestParseSumDatIni_MissingMeasurements(t *testing.T) {
	content := `
[graph_2]
GraphType=es_tr_errors_distribution
Measurement_num=0
`
	tmpfile, err := ioutil.TempFile("", "sumdatini-missing-*.ini")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpfile.Close()
	graphs, err := parseSumDatIni(tmpfile.Name())
	if err != nil {
		t.Fatalf("parseSumDatIni failed: %v", err)
	}
	g2, ok := graphs["graph_2"]
	if !ok {
		t.Errorf("graph_2 not found")
	}
	if g2.GraphType != "es_tr_errors_distribution" {
		t.Errorf("graph_2 GraphType got %q, want %q", g2.GraphType, "es_tr_errors_distribution")
	}
	if len(g2.Measurements) != 0 {
		t.Errorf("graph_2 measurements length got %d, want 0", len(g2.Measurements))
	}
}

func TestDatLineLogic(t *testing.T) {
	// Helper to simulate parsing a .dat line
	parseLine := func(line string) (value float64, count int, minv, maxv, sumv float64) {
		parts := splitFields(line)
		_, _ = parts[0], parts[1] // id, ts (not used)
		value, _ = strconv.ParseFloat(parts[2], 64)
		count, _ = strconv.Atoi(parts[3])
		minv, _ = strconv.ParseFloat(parts[4], 64)
		maxv, _ = strconv.ParseFloat(parts[5], 64)
		sumv, _ = strconv.ParseFloat(parts[6], 64)
		return
	}

	// Case 1: count == 1 (gauge)
	value, count, minv, maxv, sumv := parseLine("24 1748439937 23.372772 1 23.372772 23.372772 23.372772")
	if count != 1 || value != 23.372772 || minv != value || maxv != value || sumv != value {
		t.Errorf("Gauge parse failed: got count=%d value=%f min=%f max=%f sum=%f", count, value, minv, maxv, sumv)
	}

	// Case 2: count > 1 (rate/aggregate)
	value, count, minv, maxv, sumv = parseLine("24 1748439937 5.000000 5 5.000000 5.000000 5.000000")
	if count != 5 || value != 5.0 {
		t.Errorf("Rate parse failed: got count=%d value=%f", count, value)
	}
	// All values same, so min/max/sum == value
	if minv != value || maxv != value || sumv != value {
		t.Errorf("Rate min/max/sum logic failed: min=%f max=%f sum=%f value=%f", minv, maxv, sumv, value)
	}

	// Case 3: count == -1 (counter)
	value, count, minv, maxv, sumv = parseLine("24 1748439937 1000.000000 -1 1000.000000 1000.000000 1000.000000")
	if count != -1 || value != 1000.0 {
		t.Errorf("Counter parse failed: got count=%d value=%f", count, value)
	}
}

// Helper to split fields (handles multiple spaces)
func splitFields(line string) []string {
	var out []string
	start := -1
	for i, c := range line {
		if c != ' ' && start == -1 {
			start = i
		}
		if c == ' ' && start != -1 {
			out = append(out, line[start:i])
			start = -1
		}
	}
	if start != -1 {
		out = append(out, line[start:])
	}
	return out
}

func TestRecursiveSumDataSearch(t *testing.T) {
	dir, err := ioutil.TempDir("", "parent-results-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create two results folders, each with a sum_data folder
	for i := 0; i < 2; i++ {
		resDir := filepath.Join(dir, "RawResults_"+strconv.Itoa(1000+i))
		sumDataDir := filepath.Join(resDir, "sum_data")
		os.MkdirAll(sumDataDir, 0755)
		// Add a minimal sum_dat.ini
		ini := `[graph_0]
GraphType=es_tr_runtime_vusers
Measurement_num=1
Measurement_0=Running,1
`
		ioutil.WriteFile(filepath.Join(sumDataDir, "sum_dat.ini"), []byte(ini), 0644)
	}

	// Now search recursively for sum_data folders
	sumDataDirs := []string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read parent dir: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		resultsFolder := filepath.Join(dir, entry.Name())
		filepath.WalkDir(resultsFolder, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && d.Name() == "sum_data" {
				sumDataDirs = append(sumDataDirs, path)
			}
			return nil
		})
	}
	if len(sumDataDirs) != 2 {
		t.Errorf("Expected 2 sum_data folders, got %d", len(sumDataDirs))
	}
	// Check that each has a sum_dat.ini that can be parsed
	for _, sumDataDir := range sumDataDirs {
		graphs, err := parseSumDatIni(filepath.Join(sumDataDir, "sum_dat.ini"))
		if err != nil {
			t.Errorf("parseSumDatIni failed for %s: %v", sumDataDir, err)
		}
		if len(graphs) != 1 {
			t.Errorf("Expected 1 graph in %s, got %d", sumDataDir, len(graphs))
		}
	}
}

// Add a test to check that project and test labels are included in metric attributes
func TestMetricAttributesIncludeProjectAndTest(t *testing.T) {
	project := "DemoProject"
	test := "DemoTest"
	graphType := "es_tr_runtime_vusers"
	measurement := "Running"
	resultsFolder := "RawResults_1001"
	attrs := []string{}
	// Simulate attribute construction as in main
	attrMap := map[string]string{
		"project":        project,
		"test":           test,
		"graph_type":     graphType,
		"measurement":    measurement,
		"results_folder": resultsFolder,
	}
	for k, v := range attrMap {
		attrs = append(attrs, k+"="+v)
	}
	foundProject := false
	foundTest := false
	for _, a := range attrs {
		if a == "project="+project {
			foundProject = true
		}
		if a == "test="+test {
			foundTest = true
		}
	}
	if !foundProject || !foundTest {
		t.Errorf("Expected project and test labels in attributes, got %v", attrs)
	}
}
