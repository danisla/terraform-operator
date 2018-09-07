package test

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const (
	defaultImage                = "gcr.io/cloud-solutions-group/terraform-pod:latest"
	defaultGoogleProviderSecret = "tf-provider-google"
	defaultBucketPrefix         = "terraform"
	defaultTFSpecFile           = "tfspec.tpl.yaml"
	defaultTFSourcePath         = "testdata/tfsource.tf"
)

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

type tfSpecData struct {
	Kind                     string
	Name                     string
	Image                    string
	ConfigMapSources         []string
	BackendBucket            string
	BucketPrefix             string
	GoogleProviderSecretName string
	TFVarsMap                map[string]string
}

func testMakeTF(t *testing.T, data tfSpecData) string {

	templateSpec := helperLoadBytes(t, defaultTFSpecFile)
	tmpl, err := template.New("tf.yaml").Funcs(template.FuncMap{"StringsJoin": strings.Join}).Parse(string(templateSpec))
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
	return fmt.Sprintf("gs://%s-terraform-operator", projectID), nil
}

func getTFSource() (string, error) {
	data, err := ioutil.ReadFile("artifacts/tfsources.tf")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func testRunCmd(t *testing.T, cmdStr string) {
	var stderr bytes.Buffer
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run kubectl: %s\n%v", stderr.String(), err)
	}
}

func testApplyTFSourceConfigMap(t *testing.T, namespace, name string) {
	cmdStr := fmt.Sprintf("kubectl -n %s create --save-config=true configmap %s --from-file=main.tf=%s --dry-run -o yaml | kubectl apply -f -", namespace, name, defaultTFSourcePath)
	testRunCmd(t, cmdStr)
}

func testDeleteTFSourceConfigMap(t *testing.T, namespace, name string) {
	cmdStr := fmt.Sprintf("kubectl -n %s delete configmap %s", namespace, name)
	testRunCmd(t, cmdStr)
}
