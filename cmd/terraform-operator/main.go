package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"

	tfdriverv1 "github.com/danisla/terraform-operator/pkg/tfdriver"
	tfv1 "github.com/danisla/terraform-operator/pkg/types"
)

var (
	config         Config
	tfDriverConfig tfdriverv1.TerraformDriverConfig
)

func init() {
	config = Config{
		Project:    "", // Derived from instance metadata server
		ProjectNum: "", // Derived from instance metadata server
	}

	if err := config.loadAndValidate(); err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	tfDriverConfig = tfdriverv1.TerraformDriverConfig{}

	if err := tfDriverConfig.LoadAndValidate(config.Project); err != nil {
		log.Fatalf("Failed to load terraform driver config: %v", err)
	}
}

func main() {
	http.HandleFunc("/healthz", healthzHandler())
	http.HandleFunc("/", webhookHandler())

	log.Printf("[INFO] Initialized controller on port 80\n")
	log.Fatal(http.ListenAndServe(":80", nil))
}

func healthzHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK\n")
	}
}

func webhookHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var req SyncRequest
		var desiredStatus *tfv1.TerraformOperatorStatus
		var desiredChildren *[]interface{}
		var parentType ParentType

		if r.Method != "POST" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Unsupported method\n")
			return
		}

		if os.Getenv("HTTP_DEBUG") != "" {
			log.Printf("---HTTP REQUEST %s %s ---", r.Method, r.URL.String())
			reqDump, _ := httputil.DumpRequest(r, true)
			log.Print(string(reqDump))
		}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("[ERROR] Could not parse SyncRequest: %v", err)
			return
		}

		switch tfv1.TFKind(req.Parent.Kind) {
		case tfv1.TFKindPlan:
			parentType = ParentPlan
		case tfv1.TFKindApply:
			parentType = ParentApply
		case tfv1.TFKindDestroy:
			parentType = ParentDestroy
		}
		desiredStatus, desiredChildren, err = sync(parentType, &req.Parent, &req.Children)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("[ERROR] Could not sync state: %v", err)
		}

		resp := SyncResponse{
			Status:   *desiredStatus,
			Children: *desiredChildren,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("[ERROR] Could not generate SyncResponse: %v", err)
			return
		}
		w.Write(data)

		if os.Getenv("HTTP_DEBUG") != "" {
			log.Printf("---JSON RESPONSE %s %s ---", r.Method, r.URL.String())
			log.Println(string(data))
		}
	}
}
