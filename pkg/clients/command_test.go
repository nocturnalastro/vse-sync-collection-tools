//

//

// Copyright 2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clients_test

import (
	"errors"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeK8s "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/clients"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/testutils"
)

var notATestPod = &v1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:        "NotATestPod-3989",
		Namespace:   "TestNamespace",
		Annotations: map[string]string{},
	},
}
var testPod = &v1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:        "TestPod-8292",
		Namespace:   "TestNamespace",
		Annotations: map[string]string{},
	},
}

var _ = Describe("NewContainerContext", func() {
	var clientset *clients.Clientset
	BeforeEach(func() {
		clients.ClearClientSet()
		clientset = clients.GetClientset(kubeconfigPath)
	})

	When("A ContainerContext is requested for a pod which DOES NOT exist", func() {
		It("should return an error ", func() {
			fakeK8sClient := fakeK8s.NewSimpleClientset(notATestPod)
			clientset.K8sClient = fakeK8sClient

			_, err := clients.NewContainerContext(clientset, "TestNamespace", "Test", "TestContainer")
			Expect(err).To(HaveOccurred())
		})
	})
	When("A ContainerContext is requested for a pod which DOES exist", func() {
		It("should return the context for that pod", func() {
			fakeK8sClient := fakeK8s.NewSimpleClientset(notATestPod, testPod)
			clientset.K8sClient = fakeK8sClient

			ctx, err := clients.NewContainerContext(clientset, "TestNamespace", "Test", "TestContainer")
			Expect(err).NotTo(HaveOccurred())
			Expect(ctx.GetNamespace()).To(Equal("TestNamespace"))
			Expect(ctx.GetContainerName()).To(Equal("TestContainer"))
			Expect(ctx.GetPodName()).To(Equal("TestPod-8292"))
		})
	})

})

var _ = Describe("ExecCommandContainer", func() {
	var clientset *clients.Clientset
	var fakeK8sClient *fakeK8s.Clientset
	var fakeRestClient *rest.RESTClient
	BeforeEach(func() {
		clients.ClearClientSet()
		clientset = clients.GetClientset(kubeconfigPath)
		fakeK8sClient = fakeK8s.NewSimpleClientset(testPod)
		fakeRestClient = &rest.RESTClient{}

		clientset.K8sClient = fakeK8sClient
		clientset.K8sRestClient = fakeRestClient

	})

	When("given a pod", func() {
		It("should exec the command and return the std buffers", func() {
			expectedStdOut := "my test command stdout"
			expectedStdErr := "my test command stderr"
			responder := func(method string, url *url.URL) ([]byte, []byte, error) {
				return []byte(expectedStdOut), []byte(expectedStdErr), nil
			}
			clients.NewSPDYExecutor = testutils.NewFakeNewSPDYExecutor(responder, nil)
			ctx, _ := clients.NewContainerContext(clientset, "TestNamespace", "Test", "TestContainer")
			cmd := []string{"my", "test", "command"}
			stdout, stderr, err := clientset.ExecCommandContainer(ctx, cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(Equal(expectedStdOut))
			Expect(stderr).To(Equal(expectedStdErr))
		})
	})

	//nolint:dupl //it is incorrectly saying that this is a duplicate despite the aguments being in a different order
	When("NewSPDYExecutor fails", func() {
		It("should return an error", func() {
			expectedStdOut := ""
			expectedStdErr := ""
			expectedErr := errors.New("Something went horribly wrong when creating the executor")
			responder := func(method string, url *url.URL) ([]byte, []byte, error) {
				return []byte(expectedStdOut), []byte(expectedStdErr), nil
			}
			clients.NewSPDYExecutor = testutils.NewFakeNewSPDYExecutor(responder, expectedErr)
			ctx, _ := clients.NewContainerContext(clientset, "TestNamespace", "Test", "TestContainer")
			cmd := []string{"my", "test", "command"}
			stdout, stderr, err := clientset.ExecCommandContainer(ctx, cmd)
			Expect(err).To(HaveOccurred())
			Expect(expectedErr.Error()).To(ContainSubstring(expectedErr.Error()))
			Expect(stdout).To(Equal(expectedStdOut))
			Expect(stderr).To(Equal(expectedStdErr))
		})
	})
	//nolint:dupl //it is incorrectly saying that this is a duplicate despite the aguments being in a different order
	When("SteamWithContext fails", func() {
		It("should return an error", func() {
			expectedStdOut := ""
			expectedStdErr := ""
			expectedErr := errors.New("Something went horribly wrong with the stream")
			responder := func(method string, url *url.URL) ([]byte, []byte, error) {
				return []byte(expectedStdOut), []byte(expectedStdErr), expectedErr
			}
			clients.NewSPDYExecutor = testutils.NewFakeNewSPDYExecutor(responder, nil)
			ctx, _ := clients.NewContainerContext(clientset, "TestNamespace", "Test", "TestContainer")
			cmd := []string{"my", "test", "command"}
			stdout, stderr, err := clientset.ExecCommandContainer(ctx, cmd)
			Expect(err).To(HaveOccurred())
			Expect(expectedErr.Error()).To(ContainSubstring(expectedErr.Error()))
			Expect(stdout).To(Equal(expectedStdOut))
			Expect(stderr).To(Equal(expectedStdErr))
		})
	})
})
