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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type tfSpecFromData struct {
	Kind         TFKind
	Name         string
	TFPlan       string
	TFApply      string
	TFDestroy    string
	WaitForReady bool
}

type tfSpecData struct {
	Kind                     TFKind
	Name                     string
	Image                    string
	BackendBucket            string
	BucketPrefix             string
	ConfigMapSources         []string
	EmbeddedSources          []string
	TFSources                []TFSource
	GoogleProviderSecretName string
	TFVars                   map[string]string
	TFPlan                   string
	TFVarsFrom               []TFSource
	TFInputs                 []TFInput
}

type TFSource struct {
	TFApply string
	TFPlan  string
}

type TFInput struct {
	Name   string
	VarMap []InputVar
}

type InputVar struct {
	Source string
	Dest   string
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
	PodName    string               `json:"podName"`
	PodStatus  string               `json:"podStatus"`
	Outputs    []TerraformOutputVar `json:"outputs,omitempty"`
	Conditions []Condition          `json:"conditions,omitempty"`
}

type ConditionType string

const (
	ConditionSpecFromReady       ConditionType = "SpecFromReady"
	ConditionProviderConfigReady ConditionType = "ProviderConfigReady"
	ConditionSourceReady         ConditionType = "ConfigSourceReady"
	ConditionTFInputsReady       ConditionType = "TFInputsReady"
	ConditionVarsFromReady       ConditionType = "TFVarsFromReady"
	ConditionPlanReady           ConditionType = "TFPlanReady"
	ConditionPodComplete         ConditionType = "TFPodComplete"
	ConditionReady               ConditionType = "Ready"
)

// Condition defines the format for a status condition element.
type Condition struct {
	Type               ConditionType   `json:"type"`
	Status             ConditionStatus `json:"status"`
	LastProbeTime      metav1.Time     `json:"lastProbeTime,omitempty"`
	LastTransitionTime metav1.Time     `json:"lastTransitionTime,omitempty"`
	Reason             string          `json:"reason,omitempty"`
	Message            string          `json:"message,omitempty"`
}

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

func (tf *Terraform) VerifyOutputVars(t *testing.T) {
	// Verify outputs in status.
	allFound := len(tf.Status.Outputs) > 0
	for _, v := range tf.Status.Outputs {
		if v.Name != "project" && v.Name != "region" && v.Name != "zones" && v.Name != "metadata_key" && v.Name != "metadata_key2" && v.Name != "metadata_value" {
			allFound = false
		}
	}
	assert(t, allFound, "Incomplete output vars found in status.")
}

func (tf *Terraform) VerifyConditions(t *testing.T, conditions []ConditionType) {
	assert(t, len(conditions) == len(tf.Status.Conditions), "Different number of conditions found: %d, expected: %d", len(tf.Status.Conditions), len(conditions))
	for _, condition := range conditions {
		found := false
		for _, c := range tf.Status.Conditions {
			if c.Type == condition {
				found = true
				assert(t, c.Status == "True", "condition status not True: %s", c.Type)
				t.Logf("Condition %s PASSED", condition)
				break
			}
		}
		assert(t, found, "Condition not found in status: %s", condition)
	}
}

const (
	defaultImage                = "gcr.io/cloud-solutions-group/terraform-pod:latest"
	defaultGoogleProviderSecret = "tf-provider-google"
	defaultBucketPrefix         = "terraform"
	defaultTFSpecFile           = "tfspec.tpl.yaml"
	defaultTFSpecFromFile       = "tfspecfrom.tpl.yaml"
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
var backendBucket string
var bucketPrefix string

func init() {
	flag.StringVar(&namespace, "namespace", "default", "namespace to deploy to.")
	flag.StringVar(&backendBucket, "bucket", "", "bucket to save remote state to. Default is the PROJECT_ID-terraform-operator")
	flag.StringVar(&bucketPrefix, "bucketPrefix", "terraform", "path prefix in bucket to save remote state to.")
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
		data.BucketPrefix = bucketPrefix
	}

	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		t.Fatalf("err: %s", err)
	}

	return b.String()
}

func testMakeTFSpecFrom(t *testing.T, data tfSpecFromData) string {
	templateSpec := helperLoadBytes(t, defaultTFSpecFromFile)
	tmpl, err := template.New("tf.yaml").Funcs(template.FuncMap{"StringsJoin": strings.Join}).Funcs(sprig.TxtFuncMap()).Parse(string(templateSpec))
	if err != nil {
		t.Fatalf("err: %s", err)
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

func testWaitTF(t *testing.T, kind TFKind, namespace, name string) Terraform {
	var tf Terraform
	maxTime := time.Now().Add(time.Minute * time.Duration(timeout))
	for time.Now().Before(maxTime) {
		tf = testGetTF(t, kind, namespace, name)
		if tf.Status.PodStatus == "COMPLETED" {
			fmt.Printf("%s/%s pod: %s %s\n", kind, name, tf.Status.PodName, tf.Status.PodStatus)
			break
		} else {
			fmt.Printf("Waiting for %s/%s pod: %s %s\n", kind, name, tf.Status.PodName, tf.Status.PodStatus)
			time.Sleep(time.Second * time.Duration(5))
		}
	}
	return tf
}

func testVerifyOutputVars(t *testing.T, namespace, name string) {
	tf := testGetTF(t, TFKindApply, namespace, name)
	tf.VerifyOutputVars(t)
}
