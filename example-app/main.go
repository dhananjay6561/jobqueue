package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

// Minimal example showing how an external app integrates with JobQueue.
//
// Flow:
//   POST /register  →  enqueue a send_email job  →  return instantly
//   JobQueue worker picks up the job and processes it in the background.
//
// Run JobQueue first (./run.sh), then:
//   go run example-app/main.go
//   curl "http://localhost:9091/register?email=you@example.com&name=Alice"

func jobqueueURL() string {
	if u := os.Getenv("JOBQUEUE_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

func apiKey() string {
	if k := os.Getenv("JOBQUEUE_API_KEY"); k != "" {
		return k
	}
	return ""
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	name := r.URL.Query().Get("name")

	if email == "" {
		http.Error(w, `{"error":"email param required"}`, http.StatusBadRequest)
		return
	}

	if err := enqueueWelcomeEmail(email, name); err != nil {
		log.Printf("enqueue error: %v", err)
		http.Error(w, `{"error":"failed to queue job"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "registered — welcome email is on its way",
	})
}

func enqueueWelcomeEmail(email, name string) error {
	body, _ := json.Marshal(map[string]any{
		"type":         "send_email",
		"payload":      map[string]any{"to": email, "name": name},
		"priority":     7,
		"max_attempts": 3,
		"queue_name":   "default",
	})

	req, err := http.NewRequest("POST", jobqueueURL()+"/api/v1/jobs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", apiKey())
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("jobqueue unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("jobqueue returned HTTP %d", resp.StatusCode)
	}

	log.Printf("job enqueued for %s", email)
	return nil
}

func main() {
	http.HandleFunc("/register", registerHandler)

	addr := ":9091"
	log.Printf("example app listening on %s", addr)
	log.Printf("try: curl \"http://localhost:9091/register?email=you@example.com&name=Alice\"")
	log.Fatal(http.ListenAndServe(addr, nil))
}
