/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* This test check that SecurityContext parameters specified at the
 * pod or the container level work as intended. These tests cannot be
 * run when the 'SecurityContextDeny' addmissioin controller is not used
 * so they are skipped by default.
 */

package e2e

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func scTestPod(hostIPC bool, hostPID bool) *api.Pod {
	podName := "security-context-" + string(util.NewUUID())
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:        podName,
			Labels:      map[string]string{"name": podName},
			Annotations: map[string]string{},
		},
		Spec: api.PodSpec{
			SecurityContext: &api.PodSecurityContext{
				HostIPC: hostIPC,
				HostPID: hostPID,
			},
			Containers: []api.Container{
				{
					Name:  "test-container",
					Image: "gcr.io/google_containers/busybox:1.24",
				},
			},
			RestartPolicy: api.RestartPolicyNever,
		},
	}

	return pod
}

var _ = framework.KubeDescribe("Security Context [Feature:SecurityContext]", func() {
	f := framework.NewDefaultFramework("security-context")

	It("should support pod.Spec.SecurityContext.SupplementalGroups", func() {
		pod := scTestPod(false, false)
		pod.Spec.Containers[0].Command = []string{"id", "-G"}
		pod.Spec.SecurityContext.SupplementalGroups = []int64{1234, 5678}
		groups := []string{"1234", "5678"}
		f.TestContainerOutput("pod.Spec.SecurityContext.SupplementalGroups", pod, 0, groups)
	})

	It("should support pod.Spec.SecurityContext.RunAsUser", func() {
		pod := scTestPod(false, false)
		var uid int64 = 1001
		pod.Spec.SecurityContext.RunAsUser = &uid
		pod.Spec.Containers[0].Command = []string{"sh", "-c", "id -u"}

		f.TestContainerOutput("pod.Spec.SecurityContext.RunAsUser", pod, 0, []string{
			fmt.Sprintf("%v", uid),
		})
	})

	It("should support container.SecurityContext.RunAsUser", func() {
		pod := scTestPod(false, false)
		var uid int64 = 1001
		var overrideUid int64 = 1002
		pod.Spec.SecurityContext.RunAsUser = &uid
		pod.Spec.Containers[0].SecurityContext = new(api.SecurityContext)
		pod.Spec.Containers[0].SecurityContext.RunAsUser = &overrideUid
		pod.Spec.Containers[0].Command = []string{"sh", "-c", "id -u"}

		f.TestContainerOutput("pod.Spec.SecurityContext.RunAsUser", pod, 0, []string{
			fmt.Sprintf("%v", overrideUid),
		})
	})

	It("should support volume SELinux relabeling", func() {
		testPodSELinuxLabeling(f, false, false)
	})

	It("should support volume SELinux relabeling when using hostIPC", func() {
		testPodSELinuxLabeling(f, true, false)
	})

	It("should support volume SELinux relabeling when using hostPID", func() {
		testPodSELinuxLabeling(f, false, true)
	})

	It("should support seccomp alpha unconfined annotation on the container [Feature:Seccomp]", func() {
		// TODO: port to SecurityContext as soon as seccomp is out of alpha
		pod := scTestPod(false, false)
		pod.Annotations[api.SeccompContainerAnnotationKeyPrefix+"test-container"] = "unconfined"
		pod.Annotations[api.SeccompPodAnnotationKey] = "docker/default"
		pod.Spec.Containers[0].Command = []string{"grep", "ecc", "/proc/self/status"}
		f.TestContainerOutput(api.SeccompPodAnnotationKey, pod, 0, []string{"0"}) // seccomp disabled
	})

	It("should support seccomp alpha unconfined annotation on the pod [Feature:Seccomp]", func() {
		// TODO: port to SecurityContext as soon as seccomp is out of alpha
		pod := scTestPod(false, false)
		pod.Annotations[api.SeccompPodAnnotationKey] = "unconfined"
		pod.Spec.Containers[0].Command = []string{"grep", "ecc", "/proc/self/status"}
		f.TestContainerOutput(api.SeccompPodAnnotationKey, pod, 0, []string{"0"}) // seccomp disabled
	})

	It("should support seccomp alpha docker/default annotation [Feature:Seccomp]", func() {
		// TODO: port to SecurityContext as soon as seccomp is out of alpha
		pod := scTestPod(false, false)
		pod.Annotations[api.SeccompContainerAnnotationKeyPrefix+"test-container"] = "docker/default"
		pod.Spec.Containers[0].Command = []string{"grep", "ecc", "/proc/self/status"}
		f.TestContainerOutput(api.SeccompPodAnnotationKey, pod, 0, []string{"2"}) // seccomp filtered
	})

	It("should support seccomp default which is unconfined [Feature:Seccomp]", func() {
		// TODO: port to SecurityContext as soon as seccomp is out of alpha
		pod := scTestPod(false, false)
		pod.Spec.Containers[0].Command = []string{"grep", "ecc", "/proc/self/status"}
		f.TestContainerOutput(api.SeccompPodAnnotationKey, pod, 0, []string{"0"}) // seccomp disabled
	})
})

func testPodSELinuxLabeling(f *framework.Framework, hostIPC bool, hostPID bool) {
	// Write and read a file with an empty_dir volume
	// with a pod with the MCS label s0:c0,c1
	pod := scTestPod(hostIPC, hostPID)
	volumeName := "test-volume"
	mountPath := "/mounted_volume"
	pod.Spec.Containers[0].VolumeMounts = []api.VolumeMount{
		{
			Name:      volumeName,
			MountPath: mountPath,
		},
	}
	pod.Spec.Volumes = []api.Volume{
		{
			Name: volumeName,
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{
					Medium: api.StorageMediumDefault,
				},
			},
		},
	}
	pod.Spec.SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		Level: "s0:c0,c1",
	}
	pod.Spec.Containers[0].Command = []string{"sleep", "6000"}

	client := f.Client.Pods(f.Namespace.Name)
	_, err := client.Create(pod)

	framework.ExpectNoError(err, "Error creating pod %v", pod)
	defer client.Delete(pod.Name, nil)
	framework.ExpectNoError(framework.WaitForPodRunningInNamespace(f.Client, pod.Name, f.Namespace.Name))

	testContent := "hello"
	testFilePath := mountPath + "/TEST"
	err = f.WriteFileViaContainer(pod.Name, pod.Spec.Containers[0].Name, testFilePath, testContent)
	Expect(err).To(BeNil())
	content, err := f.ReadFileViaContainer(pod.Name, pod.Spec.Containers[0].Name, testFilePath)
	Expect(err).To(BeNil())
	Expect(content).To(ContainSubstring(testContent))

	foundPod, err := f.Client.Pods(f.Namespace.Name).Get(pod.Name)
	Expect(err).NotTo(HaveOccurred())

	// Confirm that the file can be accessed from a second
	// pod using host_path with the same MCS label
	volumeHostPath := fmt.Sprintf("%s/pods/%s/volumes/kubernetes.io~empty-dir/%s", framework.TestContext.KubeVolumeDir, foundPod.UID, volumeName)
	By(fmt.Sprintf("confirming a container with the same label can read the file under --volume-dir=%s", framework.TestContext.KubeVolumeDir))
	pod = scTestPod(hostIPC, hostPID)
	pod.Spec.NodeName = foundPod.Spec.NodeName
	volumeMounts := []api.VolumeMount{
		{
			Name:      volumeName,
			MountPath: mountPath,
		},
	}
	volumes := []api.Volume{
		{
			Name: volumeName,
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: volumeHostPath,
				},
			},
		},
	}
	pod.Spec.Containers[0].VolumeMounts = volumeMounts
	pod.Spec.Volumes = volumes
	pod.Spec.Containers[0].Command = []string{"cat", testFilePath}
	pod.Spec.SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		Level: "s0:c0,c1",
	}

	f.TestContainerOutput("Pod with same MCS label reading test file", pod, 0, []string{testContent})
	// Confirm that the same pod with a different MCS
	// label cannot access the volume
	pod = scTestPod(hostIPC, hostPID)
	pod.Spec.Volumes = volumes
	pod.Spec.Containers[0].VolumeMounts = volumeMounts
	pod.Spec.Containers[0].Command = []string{"sleep", "6000"}
	pod.Spec.SecurityContext.SELinuxOptions = &api.SELinuxOptions{
		Level: "s0:c2,c3",
	}
	_, err = client.Create(pod)
	framework.ExpectNoError(err, "Error creating pod %v", pod)
	defer client.Delete(pod.Name, nil)

	err = f.WaitForPodRunning(pod.Name)
	framework.ExpectNoError(err, "Error waiting for pod to run %v", pod)

	content, err = f.ReadFileViaContainer(pod.Name, "test-container", testFilePath)
	Expect(content).NotTo(ContainSubstring(testContent))
}
