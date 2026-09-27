package handlers

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestBuildPodDetail(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "a", Env: []corev1.EnvVar{
					{Name: "LIT", Value: "s3cret"},
					{Name: "PW", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "db"}, Key: "password"}}},
				}},
				{Name: "b", LivenessProbe: &corev1.Probe{
					ProbeHandler:     corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(8080)}},
					PeriodSeconds:    10,
					FailureThreshold: 3,
				}},
			},
			Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
		},
		Status: corev1.PodStatus{
			// Deliberately in a different order than spec.containers.
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "b", RestartCount: 4, LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}}},
				{Name: "a", Ready: true},
			},
		},
	}

	d := buildPodDetail(pod)
	a, b := d.Containers[0], d.Containers[1]

	if !a.Ready || a.RestartCount != 0 || a.LastState != nil {
		t.Errorf("container a status matched wrong entry: %+v", a)
	}
	if b.RestartCount != 4 || b.LastState == nil || b.LastState.Reason != "OOMKilled" || b.LastState.ExitCode != 137 {
		t.Errorf("container b last state wrong: %+v", b.LastState)
	}
	for _, e := range a.Env {
		if e.Value != "[hidden]" {
			t.Errorf("env value leaked: %+v", e)
		}
	}
	if a.Env[0].Source != "literal" || a.Env[1].Source != "secret db/password" {
		t.Errorf("env sources wrong: %+v", a.Env)
	}
	if got := b.Probes.Liveness; !strings.HasPrefix(got, "http-get http://:8080/healthz") || !strings.Contains(got, "period=10s") {
		t.Errorf("liveness probe = %q", got)
	}
	if got := d.Scheduling.Tolerations; len(got) != 1 || got[0] != "* op=Exists" {
		t.Errorf("tolerations = %v", got)
	}
}
