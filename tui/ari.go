package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func testARI(
	values map[string]string,
) (string, bool) {
	url := strings.TrimRight(
		values["ARI_URL"],
		"/",
	)

	url += "/ari/api-docs/resources.json"

	request, err := http.NewRequest(
		http.MethodGet,
		url,
		nil,
	)

	if err != nil {
		return "ARI failed: " + err.Error(), false
	}

	credentials := base64.StdEncoding.EncodeToString(
		[]byte(
			values["ARI_USER"] +
				":" +
				values["ARI_PASSWORD"],
		),
	)

	request.Header.Set(
		"Authorization",
		"Basic "+credentials,
	)

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	response, err := client.Do(request)

	if err != nil {
		return "ARI failed: " + err.Error(), false
	}

	defer response.Body.Close()

	if response.StatusCode >= 200 &&
		response.StatusCode < 300 {
		return fmt.Sprintf(
			"ARI ONLINE - HTTP %d",
			response.StatusCode,
		), true
	}

	return fmt.Sprintf(
		"ARI failed - HTTP %d",
		response.StatusCode,
	), false
}

type failedARIError struct{}

func (failedARIError) Error() string { return "ARI test failed" }

func statusError(ok bool) error {
	if ok {
		return nil
	}
	return failedARIError{}
}
