package controllers

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/route/v1"
	"github.com/prometheus/common/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = Describe("Pod controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		PodName      = "test-driver"
		serviceName  = PodName + "-svc"
		routeName    = PodName + "-ingress"
		PodNamespace = "spark-controller-tests"
		timeout      = time.Second * 30
		interval     = time.Millisecond * 250
	)

	Context("When Pod is running with proper label", func() {

		It("Should create a new Service and a Route ", func() {

			By("By creating a new Pod")
			ctx := context.Background()
			sparkPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PodName,
					Namespace: PodNamespace,
					Labels: map[string]string{
						"spark-role": "driver",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container-test",
							Image: "openshift/hello-openshift",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, sparkPod)).Should(Succeed())
			createdService := &corev1.Service{}
			createdRoute := &v1.Route{}
			// We'll need to retry getting this newly created CronJob, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: PodNamespace,
					Name:      serviceName,
				}, createdService)
				if err != nil {
					log.Error(err)
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: routeName, Namespace: PodNamespace}, createdRoute)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			// Let's make sure our Schedule string value was properly converted/handled.
			Expect(createdService).Should(Not(Equal(nil)))
			Expect(createdRoute).Should(Not(Equal(nil)))
		})
	})

	//Context("When deleting a running Pod with proper label", func() {
	//	It("Should delete the Service and the Route owned by the Pod", func() {
	//		By("By deleting the Pod")
	//		ctx := context.Background()
	//		pod := &corev1.Pod{}
	//		//service := &corev1.Service{}
	//		//route := &v1.Route{}
	//		namespaceName := types.NamespacedName{
	//			Namespace: PodNamespace,
	//			Name:      PodName,
	//		}
	//		Expect(k8sClient.Get(ctx, namespaceName , pod)).Should(Succeed())
	//		//Expect(k8sClient.Delete(ctx, pod))
	//		Eventually(func() bool {
	//			//err := k8sClient.Get(ctx, namespaceName, service)
	//			//if err != nil {
	//			//	log.Error(err)
	//			//	return false
	//			//}
	//
	//			return true
	//		}, timeout, interval).Should(BeTrue())
	//
	//
	//	})
	//})
})
