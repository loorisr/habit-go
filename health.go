package main

import (
	"fmt"
	"net/http"
	"time"
)

// checkHealth is used by `habit-go -healthcheck` (D5). The image is FROM scratch,
// so there is no shell or wget: the binary checks itself.
func checkHealth(url string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health: status %d", resp.StatusCode)
	}
	return nil
}
