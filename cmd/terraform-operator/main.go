package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

var (
	config Config
)

func init() {
	config = Config{
		Project:    "", // Derived from instance metadata server
		ProjectNum: "", // Derived from instance metadata server
	}

	if err := config.loadAndValidate(); err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
}

func myLog(parent *Terraform, level, msg string) {
	log.Printf("[%s][%s][%s] %s", level, parent.Kind, parent.Name, msg)
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
		if r.Method != "POST" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Unsupported method\n")
			return
		}

		var req SyncRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("[ERROR] Could not parse SyncRequest: %v", err)
			return
		}

		var err error
		var desiredStatus *TerraformControllerStatus
		var desiredChildren *[]interface{}
		switch r.URL.Path {
		case "/sync-plan":
			desiredStatus, desiredChildren, err = sync(ParentPlan, &req.Parent, &req.Children)
		case "/sync-apply":
			desiredStatus, desiredChildren, err = sync(ParentApply, &req.Parent, &req.Children)
		case "/sync-destroy":
			desiredStatus, desiredChildren, err = sync(ParentDestroy, &req.Parent, &req.Children)
		}

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
		fmt.Fprintf(w, string(data))
	}
}
