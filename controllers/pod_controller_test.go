package controllers

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = Describe("Pod controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		PodName      = "test-cronjob"
		PodNamespace = "test-spark-controller"
		timeout      = time.Second * 10
		interval     = time.Millisecond * 250
	)

	Context("When Pod is running with proper label", func() {

		It("Should create a new Service and a Route ", func() {
			By("By creating a new Servcie and Route")
			ctx := context.Background()
			sparkPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "spark-controller-tests",
					Labels: map[string]string{
						"spark-role": "driver",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container-test",
							Image: "openshift/example",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, sparkPod)).Should(Succeed())
			podLookupKey := types.NamespacedName{Name: PodName, Namespace: PodNamespace}
			createdService := &corev1.Service{}

			// We'll need to retry getting this newly created CronJob, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, podLookupKey, createdService)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			// Let's make sure our Schedule string value was properly converted/handled.
			Expect(createdService).Should(Not(Equal(nil)))
		})
	})
})
