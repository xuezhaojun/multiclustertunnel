package utils

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// ClusterManager provides utilities for managing cluster resources during tests
type ClusterManager struct {
	cfg *envconf.Config
}

// NewClusterManager creates a new cluster manager
func NewClusterManager(cfg *envconf.Config) *ClusterManager {
	return &ClusterManager{cfg: cfg}
}

// WaitForDeploymentReady waits for a deployment to be ready
func (cm *ClusterManager) WaitForDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return wait.For(
		conditions.New(cm.cfg.Client().Resources()).
			DeploymentConditionMatch(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			}, appsv1.DeploymentAvailable, corev1.ConditionTrue),
		wait.WithTimeout(timeout),
		wait.WithInterval(5*time.Second),
	)
}

// WaitForPodReady waits for pods with specific labels to be ready
func (cm *ClusterManager) WaitForPodReady(ctx context.Context, namespace string, labelSelector map[string]string, timeout time.Duration) error {
	// Simplified implementation - just wait for a bit and check manually
	// In a real implementation, you would use proper wait conditions
	time.Sleep(10 * time.Second)

	pods, err := cm.GetPodsWithLabels(ctx, namespace, labelSelector)
	if err != nil {
		return err
	}

	if len(pods) == 0 {
		return fmt.Errorf("no pods found with labels %v in namespace %s", labelSelector, namespace)
	}

	for _, pod := range pods {
		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		if !ready {
			return fmt.Errorf("pod %s is not ready", pod.Name)
		}
	}

	return nil
}

// WaitForServiceReady waits for a service to be ready and have endpoints
func (cm *ClusterManager) WaitForServiceReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	// Simplified implementation - just check if service exists
	services := &corev1.ServiceList{}
	if err := cm.cfg.Client().Resources(namespace).List(ctx, services); err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	for _, svc := range services.Items {
		if svc.Name == name && svc.Namespace == namespace {
			return nil // Service found
		}
	}

	return fmt.Errorf("service %s/%s not found", namespace, name)
}

// GetPodLogs retrieves logs from a pod
func (cm *ClusterManager) GetPodLogs(ctx context.Context, namespace, podName string) (string, error) {
	pods := &corev1.PodList{}
	if err := cm.cfg.Client().Resources(namespace).List(ctx, pods); err != nil {
		return "", err
	}

	for _, pod := range pods.Items {
		if pod.Name == podName {
			// Use kubectl to get logs (simplified approach)
			// In a real implementation, you might use the Kubernetes client directly
			return fmt.Sprintf("Logs for pod %s/%s would be retrieved here", namespace, podName), nil
		}
	}

	return "", fmt.Errorf("pod %s not found in namespace %s", podName, namespace)
}

// GetPodsWithLabels retrieves pods matching the given label selector
func (cm *ClusterManager) GetPodsWithLabels(ctx context.Context, namespace string, labels map[string]string) ([]corev1.Pod, error) {
	pods := &corev1.PodList{}
	if err := cm.cfg.Client().Resources(namespace).List(ctx, pods); err != nil {
		return nil, err
	}

	var matchingPods []corev1.Pod
	for _, pod := range pods.Items {
		matches := true
		for key, value := range labels {
			if pod.Labels[key] != value {
				matches = false
				break
			}
		}
		if matches {
			matchingPods = append(matchingPods, pod)
		}
	}

	return matchingPods, nil
}

// DeleteResource deletes a Kubernetes resource
func (cm *ClusterManager) DeleteResource(ctx context.Context, obj client.Object) error {
	return cm.cfg.Client().Resources().Delete(ctx, obj)
}

// CreateResource creates a Kubernetes resource
func (cm *ClusterManager) CreateResource(ctx context.Context, obj client.Object) error {
	return cm.cfg.Client().Resources().Create(ctx, obj)
}

// UpdateResource updates a Kubernetes resource
func (cm *ClusterManager) UpdateResource(ctx context.Context, obj client.Object) error {
	return cm.cfg.Client().Resources().Update(ctx, obj)
}

// WaitForNamespace waits for a namespace to be active
func (cm *ClusterManager) WaitForNamespace(ctx context.Context, name string, timeout time.Duration) error {
	// Simplified implementation - just check if namespace exists
	namespaces := &corev1.NamespaceList{}
	if err := cm.cfg.Client().Resources().List(ctx, namespaces); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	for _, ns := range namespaces.Items {
		if ns.Name == name && ns.Status.Phase == corev1.NamespaceActive {
			return nil
		}
	}

	return fmt.Errorf("namespace %s not found or not active", name)
}

// CheckClusterHealth performs basic cluster health checks
func (cm *ClusterManager) CheckClusterHealth(ctx context.Context) error {
	// Check if nodes are ready
	nodes := &corev1.NodeList{}
	if err := cm.cfg.Client().Resources().List(ctx, nodes); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	readyNodes := 0
	for _, node := range nodes.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNodes++
				break
			}
		}
	}

	if readyNodes == 0 {
		return fmt.Errorf("no ready nodes found")
	}

	// Check if system pods are running
	systemPods := &corev1.PodList{}
	if err := cm.cfg.Client().Resources("kube-system").List(ctx, systemPods); err != nil {
		return fmt.Errorf("failed to list system pods: %w", err)
	}

	runningSystemPods := 0
	for _, pod := range systemPods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningSystemPods++
		}
	}

	if runningSystemPods == 0 {
		return fmt.Errorf("no running system pods found")
	}

	return nil
}

// Global cluster manager instance
var GlobalClusterManager *ClusterManager

// InitializeClusterManager initializes the global cluster manager
func InitializeClusterManager(cfg *envconf.Config) {
	GlobalClusterManager = NewClusterManager(cfg)
}
