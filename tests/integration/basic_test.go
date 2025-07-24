package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Basic Connectivity", func() {
	var framework *TestFramework

	BeforeEach(func() {
		framework = NewTestFrameworkWithGinkgo(false) // Start with insecure for simplicity
		Expect(framework.Setup()).To(Succeed())
	})

	AfterEach(func() {
		if framework != nil {
			framework.Cleanup()
		}
	})

	It("should establish basic tunnel connectivity", func() {
		// Create a mock backend server
		mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello from backend"))
		})
		Expect(err).NotTo(HaveOccurred())

		// Create an agent that routes to the mock server
		err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// First, test if Hub HTTP server is reachable
		hubHTTPAddr := framework.GetHubHTTPAddr()
		GinkgoLogr.Info("Testing Hub HTTP server", "address", hubHTTPAddr)

		// Try a simple health check first
		healthURL := fmt.Sprintf("http://%s/health", hubHTTPAddr)
		GinkgoLogr.Info("Sending health check", "url", healthURL)
		healthResp, err := http.Get(healthURL)
		if err != nil {
			GinkgoLogr.Info("Health check failed", "error", err)
		} else {
			GinkgoLogr.Info("Health check response", "status", healthResp.StatusCode)
			healthResp.Body.Close()
		}

		// Send a request through the tunnel using a new HTTP client
		requestURL := fmt.Sprintf("http://%s/test-cluster/api/v1/test", hubHTTPAddr)
		GinkgoLogr.Info("Sending HTTP request", "url", requestURL)

		// Use a new HTTP client to avoid connection reuse issues
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		GinkgoLogr.Info("About to send HTTP request...")
		resp, err := client.Get(requestURL)
		GinkgoLogr.Info("HTTP request completed", "error", err)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		GinkgoLogr.Info("HTTP response", "status", resp.StatusCode)
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		GinkgoLogr.Info("About to read response body...")
		body, err := io.ReadAll(resp.Body)
		GinkgoLogr.Info("Response body read completed", "error", err)
		Expect(err).NotTo(HaveOccurred())
		GinkgoLogr.Info("Response body", "body", string(body))
		Expect(string(body)).To(Equal("Hello from backend"))

		// Verify the request was received by the backend
		GinkgoLogr.Info("About to check mock server requests...")
		requests := mockServer.GetRequests()
		GinkgoLogr.Info("Mock server received requests", "count", len(requests))
		Expect(requests).To(HaveLen(1))
		if len(requests) > 0 {
			GinkgoLogr.Info("First request", "method", requests[0].Method, "path", requests[0].Path)
			Expect(requests[0].Method).To(Equal("GET"))
			Expect(requests[0].Path).To(ContainSubstring("/api/v1/test"))
		}
	})

	It("should handle multiple concurrent requests", func() {
		// Create a mock backend server that responds with request ID
		requestCount := 0
		var mu sync.Mutex
		mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestCount++
			id := requestCount
			mu.Unlock()

			// Simulate some processing time
			time.Sleep(100 * time.Millisecond)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("Response %d", id)))
		})
		Expect(err).NotTo(HaveOccurred())

		// Create an agent
		err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// Send multiple concurrent requests
		const numRequests = 10
		var wg sync.WaitGroup
		responses := make([]string, numRequests)
		errors := make([]error, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				resp, err := http.Get(fmt.Sprintf("http://%s/test-cluster/request/%d", framework.GetHubHTTPAddr(), index))
				if err != nil {
					errors[index] = err
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errors[index] = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
					return
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					errors[index] = err
					return
				}

				responses[index] = string(body)
			}(i)
		}

		wg.Wait()

		// Check that all requests succeeded
		for i, err := range errors {
			Expect(err).NotTo(HaveOccurred(), "Request %d failed", i)
		}

		// Check that all responses are unique (indicating proper multiplexing)
		responseSet := make(map[string]bool)
		for i, response := range responses {
			if response != "" {
				Expect(responseSet[response]).To(BeFalse(), "Duplicate response %s for request %d", response, i)
				responseSet[response] = true
			}
		}

		// Verify all requests were received by the backend
		requests := mockServer.GetRequests()
		Expect(requests).To(HaveLen(numRequests))
	})

	It("should transfer large amounts of data", func() {
		// Create large test data (1MB) to test original requirement
		largeData := make([]byte, 1024*1024)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		// Create a mock backend server that echoes the request body
		mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		})
		Expect(err).NotTo(HaveOccurred())

		// Create an agent
		err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// Send large data through the tunnel
		resp, err := http.Post(
			fmt.Sprintf("http://%s/test-cluster/upload", framework.GetHubHTTPAddr()),
			"application/octet-stream",
			bytes.NewReader(largeData),
		)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		// Read the response and verify it matches the sent data
		responseData, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())

		GinkgoLogr.Info("Data transfer sizes", "sent", len(largeData), "received", len(responseData))
		if len(responseData) > 0 {
			end := 100
			if len(responseData) < end {
				end = len(responseData)
			}
			GinkgoLogr.Info("First 100 bytes of response", "data", fmt.Sprintf("%x", responseData[:end]))
		}
		if len(largeData) > 0 {
			end := 100
			if len(largeData) < end {
				end = len(largeData)
			}
			GinkgoLogr.Info("First 100 bytes of sent data", "data", fmt.Sprintf("%x", largeData[:end]))
		}

		Expect(responseData).To(Equal(largeData), "Response data doesn't match sent data")

		// Verify the request was received by the backend
		requests := mockServer.GetRequests()
		Expect(requests).To(HaveLen(1))
		Expect(requests[0].Method).To(Equal("POST"))
		Expect(requests[0].Body).To(Equal(largeData))
	})

	It("should handle different HTTP methods", func() {
		// Create a mock backend server that echoes the method and body
		mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("Method: %s, Body: %s", r.Method, string(body))))
		})
		Expect(err).NotTo(HaveOccurred())

		// Create an agent
		err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		baseURL := fmt.Sprintf("http://%s/test-cluster/api", framework.GetHubHTTPAddr())

		testCases := []struct {
			method string
			body   string
		}{
			{"GET", ""},
			{"POST", "post data"},
			{"PUT", "put data"},
			{"DELETE", ""},
			{"PATCH", "patch data"},
		}

		for _, tc := range testCases {
			By(fmt.Sprintf("Testing %s method", tc.method))
			var req *http.Request
			var err error

			if tc.body != "" {
				req, err = http.NewRequest(tc.method, baseURL, strings.NewReader(tc.body))
			} else {
				req, err = http.NewRequest(tc.method, baseURL, nil)
			}
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			responseBody, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			expectedResponse := fmt.Sprintf("Method: %s, Body: %s", tc.method, tc.body)
			Expect(string(responseBody)).To(Equal(expectedResponse))
		}

		// Verify all requests were received by the backend
		requests := mockServer.GetRequests()
		Expect(requests).To(HaveLen(len(testCases)))
	})
})

var _ = Describe("TLS Connectivity", func() {
	var framework *TestFramework

	BeforeEach(func() {
		framework = NewTestFrameworkWithGinkgo(true) // Enable TLS
		Expect(framework.Setup()).To(Succeed())
	})

	AfterEach(func() {
		if framework != nil {
			framework.Cleanup()
		}
	})

	It("should establish TLS-enabled connectivity", func() {
		// Create a mock backend server
		mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello from TLS backend"))
		})
		Expect(err).NotTo(HaveOccurred())

		// Create an agent that routes to the mock server
		err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// Create HTTP client with TLS configuration for HTTPS requests
		client := &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: getTestClientTLSConfig(),
			},
		}

		// Send a request through the TLS tunnel using HTTPS
		requestURL := fmt.Sprintf("https://%s/test-cluster/api/v1/test", framework.GetHubHTTPAddr())
		resp, err := client.Get(requestURL)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("Hello from TLS backend"))

		// Verify the request was received by the backend
		requests := mockServer.GetRequests()
		Expect(requests).To(HaveLen(1))
	})
})
