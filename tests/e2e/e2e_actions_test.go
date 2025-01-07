//go:build e2e

/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/NVIDIA/dcgm-exporter/tests/e2e/internal/framework"
)

func shouldCreateK8SConfig() *restclient.Config {
	config, err := clientcmd.BuildConfigFromFlags("", testContext.kubeconfig)
	Expect(err).ShouldNot(HaveOccurred(), "unable to load kubeconfig from %s; err: %s",
		testContext.kubeconfig, err)
	return config
}

func shouldResolvePath() {
	var err error
	testContext.kubeconfig, err = framework.ResolvePath(testContext.kubeconfig)
	Expect(err).ShouldNot(HaveOccurred(),
		"cannot resolve path to kubeconfig: %s, err: %v", testContext.kubeconfig, err)
}

func shouldCreateNamespace(ctx context.Context, kubeClient *framework.KubeClient, labels map[string]string) {
	By(fmt.Sprintf("Creating namespace: %q started.", testContext.namespace))
	_, err := kubeClient.CreateNamespace(ctx, testContext.namespace, labels)
	Expect(err).ShouldNot(HaveOccurred(), "Creating namespace: failed")
	By(fmt.Sprintf("Creating namespace: %q completed\n", testContext.namespace))
}

func shouldCreateKubeClient(config *rest.Config) *framework.KubeClient {
	kubeClient, err := framework.NewKubeClient(config)
	Expect(err).ShouldNot(HaveOccurred(), "cannot create k8s client: %s", err)
	return kubeClient
}

func kubeConfigShouldExists() {
	if _, err := os.Stat(testContext.kubeconfig); os.IsNotExist(err) {
		Fail(fmt.Sprintf("kubeconfig file does not exist: %s", testContext.kubeconfig))
	}
}

func shouldCreateHelmClient(config *rest.Config) *framework.HelmClient {
	helmClient, err := framework.NewHelmClient(
		framework.HelmWithNamespace(testContext.namespace),
		framework.HelmWithKubeConfig(config),
		framework.HelmWithChart(testContext.chart),
	)
	Expect(err).ShouldNot(HaveOccurred(), "Creating helm client: %q failed",
		testContext.namespace)

	return helmClient
}

func shouldUninstallHelmChart(helmClient *framework.HelmClient, helmReleaseName string) {
	if helmClient != nil && helmReleaseName != "" {
		By(fmt.Sprintf("Helm chart uninstall: release %q of the helm chart: %q started.",
			helmReleaseName,
			testContext.chart))

		err := helmClient.Uninstall(helmReleaseName)
		if err != nil {
			Fail(fmt.Sprintf("Helm chart uninstall: release: %s uninstall failed with error: %v", helmReleaseName, err))
		} else {
			By(fmt.Sprintf("Helm chart uninstall: release %q of the helm chart: %q completed.",
				helmReleaseName,
				testContext.chart))
		}
	}
}

func shouldCleanupHelmClient(helmClient *framework.HelmClient) {
	if helmClient != nil {
		err := helmClient.Cleanup()
		if err != nil {
			Fail(fmt.Sprintf("Helm Client: clean up failed: %v", err))
		}
	}
}

func shouldDeleteNamespace(ctx context.Context, kubeClient *framework.KubeClient) {
	By(fmt.Sprintf("Namespace deletion: %q namespace started.", testContext.namespace))
	if kubeClient != nil {
		err := kubeClient.DeleteNamespace(ctx, testContext.namespace)
		if err != nil {
			Fail(fmt.Sprintf("Namespace deletion: Failed to delete namespace %q with error: %v", testContext.namespace,
				err))
		} else {
			By(fmt.Sprintf("Namespace deletion: %q namespace completed.\n", testContext.namespace))
		}
	}
}

func shouldCheckIfPodCreated(
	ctx context.Context, kubeClient *framework.KubeClient, labels map[string]string,
) *corev1.Pod {
	By("Pod creation verification: started")

	var dcgmExpPod *corev1.Pod

	Eventually(func(ctx context.Context) bool {
		pods, err := kubeClient.GetPodsByLabel(ctx, testContext.namespace, labels)
		if err != nil {
			Fail(fmt.Sprintf("Pod creation: Failed with error: %v", err))
			return false
		}

		if len(pods) == 1 {
			dcgmExpPod = &pods[0]
			return true
		}

		return false
	}).WithPolling(time.Second).Within(15 * time.Minute).WithContext(ctx).Should(BeTrue())

	By("Pod creation verification: completed")

	return dcgmExpPod
}

func getDefaultHelmValues() []string {
	values := []string{
		fmt.Sprintf("serviceMonitor.enabled=%v", false),
	}

	if testContext.arguments != "" {
		values = append(values, fmt.Sprintf("arguments=%s", testContext.arguments))
	}

	if testContext.imageRepository != "" {
		values = append(values, fmt.Sprintf("image.repository=%s", testContext.imageRepository))
	}

	if testContext.imageTag != "" {
		values = append(values, fmt.Sprintf("image.tag=%s", testContext.imageTag))
	}

	if testContext.runtimeClass != "" {
		values = append(values, fmt.Sprintf("runtimeClassName=%s", testContext.runtimeClass))
	}

	return values
}

func shouldCheckIfPodIsReady(ctx context.Context, kubeClient *framework.KubeClient, namespace, podName string) {
	By("Checking pod status: started")
	Eventually(func(ctx context.Context) bool {
		isReady, err := kubeClient.CheckPodStatus(ctx,
			namespace,
			podName,
			func(namespace, podName string, status corev1.PodStatus) (bool, error) {
				for _, c := range status.Conditions {
					if c.Type != corev1.PodReady {
						continue
					}
					if c.Status == corev1.ConditionTrue {
						return true, nil
					}
				}

				for _, c := range status.ContainerStatuses {
					if c.State.Waiting != nil && c.State.Waiting.Reason == "CrashLoopBackOff" {
						return false, fmt.Errorf("pod %s in namespace %s is in CrashLoopBackOff", podName, namespace)
					}
				}

				return false, nil
			})
		if err != nil {
			Fail(fmt.Sprintf("Checking pod status: Failed with error: %v", err))
		}

		return isReady
	}).WithPolling(time.Second).Within(15 * time.Minute).WithContext(ctx).Should(BeTrue())
	By("Checking pod status: completed")
}
