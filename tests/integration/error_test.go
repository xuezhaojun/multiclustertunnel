package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Error Handling", func() {
	var framework *TestFramework

	BeforeEach(func() {
		framework = NewTestFrameworkWithGinkgo(false)
		Expect(framework.Setup()).To(Succeed())
	})

	AfterEach(func() {
		if framework != nil {
			framework.Cleanup()
		}
	})

	It("should return bad request for non-existent clusters", func() {
		// Don't create any agents, so no clusters are available

		// Send a request to a non-existent cluster
		resp, err := http.Get(fmt.Sprintf("http://%s/non-existent-cluster/api/v1/test", framework.GetHubHTTPAddr()))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		// Should get a bad request error since no route is configured
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(ContainSubstring("Failed to parse cluster name and target address from request"))
	})

	It("should return bad gateway when backend service is unavailable", func() {
		// Create an agent that routes to a non-existent backend
		err := framework.CreateAgent("test-cluster", "localhost:99999") // Non-existent port
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// Send a request through the tunnel
		resp, err := http.Get(fmt.Sprintf("http://%s/test-cluster/api/v1/test", framework.GetHubHTTPAddr()))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		// Should get a bad gateway error
		Expect(resp.StatusCode).To(Equal(http.StatusBadGateway))
	})

	It("should handle request timeout scenarios", func() {
		// Create a mock backend server that hangs
		mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
			// Hang for longer than the client timeout
			time.Sleep(35 * time.Second)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Too late"))
		})
		Expect(err).NotTo(HaveOccurred())

		// Create an agent
		err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// Send a request with a short timeout
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://%s/test-cluster/api/v1/test", framework.GetHubHTTPAddr()))

		// Should get a timeout error
		if err != nil {
			// Accept various timeout error formats
			errMsg := err.Error()
			Expect(strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded")).To(BeTrue(),
				"Expected timeout error, got: %s", errMsg)
		} else {
			defer resp.Body.Close()
			// If we get a response, it should be a timeout status
			Expect(resp.StatusCode == http.StatusGatewayTimeout || resp.StatusCode == http.StatusRequestTimeout).To(BeTrue())
		}
	})

	It("should handle client request cancellation", func() {
		// Create a mock backend server that takes some time to respond
		mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
			// Wait for a bit, but not too long
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Response"))
		})
		Expect(err).NotTo(HaveOccurred())

		// Create an agent
		err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// Create a request with a context that we'll cancel
		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("http://%s/test-cluster/api/v1/test", framework.GetHubHTTPAddr()), nil)
		Expect(err).NotTo(HaveOccurred())

		// Start the request in a goroutine
		done := make(chan error, 1)
		go func() {
			client := &http.Client{}
			resp, err := client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
			done <- err
		}()

		// Cancel the request after a short delay
		time.Sleep(500 * time.Millisecond)
		cancel()

		// Wait for the request to complete
		select {
		case err := <-done:
			// Should get a context cancellation error
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("context canceled"))
		case <-time.After(5 * time.Second):
			Fail("Request didn't complete within expected time")
		}
	})

	It("should handle requests with invalid cluster names", func() {
		testCases := []struct {
			name string
			path string
		}{
			{"empty path", "/"},
			{"no cluster name", "/api/v1/test"},
			{"empty cluster name", "//api/v1/test"},
		}

		for _, tc := range testCases {
			By(fmt.Sprintf("Testing %s", tc.name))
			resp, err := http.Get(fmt.Sprintf("http://%s%s", framework.GetHubHTTPAddr(), tc.path))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			// Should get a bad request error
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.ToLower(string(body))).To(ContainSubstring("cluster"))
		}
	})

	It("should handle slow backend responses", func() {
		// Create a mock backend server that responds slowly but within timeout
		mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
			// Respond slowly but within the default timeout
			time.Sleep(1 * time.Second)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Slow response"))
		})
		Expect(err).NotTo(HaveOccurred())

		// Create an agent
		err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// Send a request with a reasonable timeout
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://%s/test-cluster/api/v1/test", framework.GetHubHTTPAddr()))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("Slow response"))
	})

	// Note: Backend error status code propagation test is disabled
	// HTTP status code propagation is currently not supported due to the use of HTTP hijacking
	// for transparent tunneling. This test is disabled until the architecture is redesigned to support
	// proper HTTP response parsing and status code forwarding.
	//
	// TODO: Implement HTTP response parsing to support status code propagation

	It("should properly clean up resources", func() {
		// Create a mock backend server
		mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		Expect(err).NotTo(HaveOccurred())

		// Create an agent
		err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// Send a request to establish connections
		resp, err := http.Get(fmt.Sprintf("http://%s/test-cluster/api/v1/test", framework.GetHubHTTPAddr()))
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()

		// Verify the request was successful
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		// The cleanup will be tested when the framework is torn down
		// We can't easily test internal resource cleanup without exposing internals,
		// but we can verify that the framework shuts down cleanly
	})

	It("should handle DNS resolution failures", func() {
		// Create an agent that routes to a non-resolvable hostname
		err := framework.CreateAgent("test-cluster", "non-existent-hostname.invalid:8080")
		Expect(err).NotTo(HaveOccurred())

		// Wait for agent to connect
		time.Sleep(500 * time.Millisecond)

		// Send a request through the tunnel
		resp, err := http.Get(fmt.Sprintf("http://%s/test-cluster/api/v1/test", framework.GetHubHTTPAddr()))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		// Should get a bad gateway error due to DNS resolution failure
		Expect(resp.StatusCode).To(Equal(http.StatusBadGateway))
	})
})
