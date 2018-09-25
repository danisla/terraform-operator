package test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
)

type tfSpecData struct {
	Kind                     TFKind
	Name                     string
	Image                    string
	ConfigMapSources         []string
	EmbeddedSources          []string
	TFSources                []map[string]string
	BackendBucket            string
	BucketPrefix             string
	GoogleProviderSecretName string
	TFVarsMap                map[string]string
	TFPlan                   string
}

type TerraformOutputVar struct {
	Name      string `json:"name,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
	Type      string `json:"type,omitempty"`
	Value     string `json:"value,omitempty"`
}

type Terraform struct {
	Status TerraformStatus `json:"status"`
}

type TerraformStatus struct {
	PodName   string               `json:"podName"`
	PodStatus string               `json:"podStatus"`
	Outputs   []TerraformOutputVar `json:"outputs,omitempty"`
}

func (tf *Terraform) VerifyOutputVars(t *testing.T) {
	// Verify outputs in status.
	allFound := len(tf.Status.Outputs) > 0
	for _, v := range tf.Status.Outputs {
		if v.Name != "project" && v.Name != "region" && v.Name != "zones" && v.Name != "metadata_key" && v.Name != "metadata_value" {
			allFound = false
		}
	}
	assert(t, allFound, "Incomplete output vars found in status.")
}

const (
	defaultImage                = "gcr.io/cloud-solutions-group/terraform-pod:latest"
	defaultGoogleProviderSecret = "tf-provider-google"
	defaultBucketPrefix         = "terraform"
	defaultTFSpecFile           = "tfspec.tpl.yaml"
	defaultTFSourcePath         = "tfsource.tf"
)

type TFKind string

const (
	TFKindPlan    TFKind = "TerraformPlan"
	TFKindApply   TFKind = "TerraformApply"
	TFKindDestroy TFKind = "TerraformDestroy"
)

var namespace string
var deleteSpec bool
var timeout int

func init() {
	flag.StringVar(&namespace, "namespace", "default", "namespace to deploy to.")
	flag.BoolVar(&deleteSpec, "delete", true, "wether to delete the applied test specs.")
	flag.IntVar(&timeout, "timeout", 10, "timeout in minutes to wait for any Terraform operation")
	flag.Parse()
}

// assert fails the test if the condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
}

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}

func helperLoadBytes(t *testing.T, name string) []byte {
	path := filepath.Join("testdata", name) // relative path
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func testMakeTF(t *testing.T, data tfSpecData) string {
	templateSpec := helperLoadBytes(t, defaultTFSpecFile)
	tmpl, err := template.New("tf.yaml").Funcs(template.FuncMap{"StringsJoin": strings.Join}).Funcs(sprig.TxtFuncMap()).Parse(string(templateSpec))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if data.Image == "" {
		data.Image = defaultImage
	}
	if data.GoogleProviderSecretName == "" {
		data.GoogleProviderSecretName = defaultGoogleProviderSecret
	}
	if data.BackendBucket == "" {
		// Generate from project ID.
		bucket, err := defaultBackendBucket()
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		data.BackendBucket = bucket
	}
	if data.BucketPrefix == "" {
		data.BucketPrefix = defaultBucketPrefix
	}

	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		t.Fatalf("err: %s", err)
	}

	return b.String()
}

func defaultBackendBucket() (string, error) {
	projectID := os.Getenv("GOOGLE_PROJECT")
	if projectID == "" {
		return "", fmt.Errorf("No GOOGLE_PROJECT found in env")
	}
	return fmt.Sprintf("%s-terraform-operator", projectID), nil
}

func getTFSource() (string, error) {
	data, err := ioutil.ReadFile("artifacts/tfsources.tf")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func testRunCmd(t *testing.T, cmdStr string, stdin string) string {
	var stderr, stdout bytes.Buffer
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	if stdin != "" {
		cmd.Stdin = bytes.NewBufferString(stdin)
	}
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run kubectl: %s\n%v", stderr.String(), err)
	}
	return stdout.String()
}

func testApplyTFSourceConfigMap(t *testing.T, namespace, name string) {
	cmdStr := fmt.Sprintf("kubectl -n %s create --save-config=true configmap %s --from-file=main.tf=%s --dry-run -o yaml | kubectl apply -f -", namespace, name, filepath.Join("testdata", defaultTFSourcePath))
	testRunCmd(t, cmdStr, "")
}

func testApply(t *testing.T, namespace, manifest string) {
	cmdStr := fmt.Sprintf("kubectl -n %s apply -f -", namespace)
	testRunCmd(t, cmdStr, manifest)
}

func testDelete(t *testing.T, namespace, manifest string) {
	if !deleteSpec {
		t.Log("Skipping deletion of manifest per flag")
	} else {
		cmdStr := fmt.Sprintf("kubectl -n %s delete -f -", namespace)
		testRunCmd(t, cmdStr, manifest)
	}
}

func testDeleteTFSourceConfigMap(t *testing.T, namespace, name string) {
	if !deleteSpec {
		t.Logf("Skipping deletion of ConfigMap/%s per flag", name)
	} else {
		cmdStr := fmt.Sprintf("kubectl -n %s delete configmap %s", namespace, name)
		testRunCmd(t, cmdStr, "")
	}
}

func testGetTF(t *testing.T, kind TFKind, namespace, name string) Terraform {
	tfJSON := testRunCmd(t, fmt.Sprintf("kubectl -n %s get %s %s -o json", namespace, kind, name), "")
	var tf Terraform
	if err := json.Unmarshal([]byte(tfJSON), &tf); err != nil {
		t.Fatalf("Failed to parse kubectl json output: %v", err)
	}
	return tf
}

func testWaitTF(t *testing.T, kind TFKind, namespace, name string) {
	maxTime := time.Now().Add(time.Minute * time.Duration(timeout))
	for time.Now().Before(maxTime) {
		tf := testGetTF(t, kind, namespace, name)
		if tf.Status.PodStatus == "COMPLETED" {
			fmt.Printf("%s/%s pod: %s %s\n", kind, name, tf.Status.PodName, tf.Status.PodStatus)
			break
		} else {
			fmt.Printf("Waiting for %s/%s pod: %s %s\n", kind, name, tf.Status.PodName, tf.Status.PodStatus)
			time.Sleep(time.Second * time.Duration(5))
		}
	}
}
