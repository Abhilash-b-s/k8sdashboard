package handlers

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// LastStateInfo describes a container's previous termination (why it restarted).
type LastStateInfo struct {
	Reason     string `json:"reason"`
	ExitCode   int32  `json:"exitCode"`
	Signal     int32  `json:"signal,omitempty"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
}

// ProbesInfo holds one-line summaries of a container's probes.
type ProbesInfo struct {
	Liveness  string `json:"liveness,omitempty"`
	Readiness string `json:"readiness,omitempty"`
	Startup   string `json:"startup,omitempty"`
}

// SchedulingInfo summarises where/how a pod may be scheduled.
type SchedulingInfo struct {
	NodeSelector  map[string]string `json:"nodeSelector,omitempty"`
	Tolerations   []string          `json:"tolerations,omitempty"`
	Affinity      []string          `json:"affinity,omitempty"`
	PriorityClass string            `json:"priorityClass,omitempty"`
	Priority      *int32            `json:"priority,omitempty"`
	SchedulerName string            `json:"schedulerName,omitempty"`
}

// PodSettingsInfo holds pod-level runtime settings.
type PodSettingsInfo struct {
	HostNetwork                   bool     `json:"hostNetwork"`
	HostPID                       bool     `json:"hostPID"`
	HostIPC                       bool     `json:"hostIPC"`
	DNSPolicy                     string   `json:"dnsPolicy,omitempty"`
	TerminationGracePeriodSeconds *int64   `json:"terminationGracePeriodSeconds,omitempty"`
	AutomountServiceAccountToken  *bool    `json:"automountServiceAccountToken,omitempty"`
	SecurityContext               []string `json:"securityContext,omitempty"`
}

// buildPodDetail converts a Pod into the detail response shared by the legacy
// and multi-cluster pod detail handlers.
func buildPodDetail(pod *corev1.Pod) PodDetailResponse {
	containers := make([]ContainerDetail, 0, len(pod.Spec.Containers))
	for _, ctr := range pod.Spec.Containers {
		containers = append(containers, buildContainerDetail(ctr, pod.Status.ContainerStatuses))
	}
	initContainers := make([]ContainerDetail, 0, len(pod.Spec.InitContainers))
	for _, ctr := range pod.Spec.InitContainers {
		initContainers = append(initContainers, buildContainerDetail(ctr, pod.Status.InitContainerStatuses))
	}

	volumes := make([]VolumeInfo, 0, len(pod.Spec.Volumes))
	for _, vol := range pod.Spec.Volumes {
		vi := VolumeInfo{Name: vol.Name, Type: "Other", Source: "-"}
		switch {
		case vol.ConfigMap != nil:
			vi.Type, vi.Source = "ConfigMap", vol.ConfigMap.Name
		case vol.Secret != nil:
			vi.Type, vi.Source = "Secret", vol.Secret.SecretName
		case vol.PersistentVolumeClaim != nil:
			vi.Type, vi.Source = "PVC", vol.PersistentVolumeClaim.ClaimName
		case vol.EmptyDir != nil:
			vi.Type = "EmptyDir"
		case vol.HostPath != nil:
			vi.Type, vi.Source = "HostPath", vol.HostPath.Path
		case vol.Projected != nil:
			vi.Type = "Projected"
		}
		volumes = append(volumes, vi)
	}

	ownerRefs := make([]OwnerReferenceInfo, 0, len(pod.OwnerReferences))
	for _, ref := range pod.OwnerReferences {
		ownerRefs = append(ownerRefs, OwnerReferenceInfo{Kind: ref.Kind, Name: ref.Name, UID: string(ref.UID)})
	}

	conditions := make([]ConditionInfo, 0, len(pod.Status.Conditions))
	for _, cond := range pod.Status.Conditions {
		conditions = append(conditions, ConditionInfo{
			Type: string(cond.Type), Status: string(cond.Status), Reason: cond.Reason, Message: cond.Message,
		})
	}

	spec := pod.Spec
	tolerations := make([]string, 0, len(spec.Tolerations))
	for _, t := range spec.Tolerations {
		tolerations = append(tolerations, tolerationString(t))
	}

	return PodDetailResponse{
		Name:            pod.Name,
		Namespace:       pod.Namespace,
		UID:             string(pod.UID),
		Node:            spec.NodeName,
		Status:          string(pod.Status.Phase),
		PodIP:           pod.Status.PodIP,
		HostIP:          pod.Status.HostIP,
		QOSClass:        string(pod.Status.QOSClass),
		ServiceAccount:  spec.ServiceAccountName,
		RestartPolicy:   string(spec.RestartPolicy),
		Labels:          pod.Labels,
		Annotations:     pod.Annotations,
		OwnerReferences: ownerRefs,
		Conditions:      conditions,
		Containers:      containers,
		InitContainers:  initContainers,
		Volumes:         volumes,
		CreatedAt:       pod.CreationTimestamp.Format("2006-01-02 15:04:05"),
		Age:             FormatAge(pod.CreationTimestamp.Time),
		Scheduling: SchedulingInfo{
			NodeSelector:  spec.NodeSelector,
			Tolerations:   tolerations,
			Affinity:      affinityStrings(spec.Affinity),
			PriorityClass: spec.PriorityClassName,
			Priority:      spec.Priority,
			SchedulerName: spec.SchedulerName,
		},
		Settings: PodSettingsInfo{
			HostNetwork:                   spec.HostNetwork,
			HostPID:                       spec.HostPID,
			HostIPC:                       spec.HostIPC,
			DNSPolicy:                     string(spec.DNSPolicy),
			TerminationGracePeriodSeconds: spec.TerminationGracePeriodSeconds,
			AutomountServiceAccountToken:  spec.AutomountServiceAccountToken,
			SecurityContext:               podSecurityStrings(spec.SecurityContext),
		},
	}
}

// buildContainerDetail matches the container's status by name (status order
// is not guaranteed to follow spec order).
func buildContainerDetail(ctr corev1.Container, statuses []corev1.ContainerStatus) ContainerDetail {
	cd := ContainerDetail{
		Name:            ctr.Name,
		Image:           ctr.Image,
		ImagePullPolicy: string(ctr.ImagePullPolicy),
		Command:         ctr.Command,
		Args:            ctr.Args,
		WorkingDir:      ctr.WorkingDir,
		Resources:       ResourceRequirements{Requests: map[string]string{}, Limits: map[string]string{}},
		Probes: ProbesInfo{
			Liveness:  probeString(ctr.LivenessProbe),
			Readiness: probeString(ctr.ReadinessProbe),
			Startup:   probeString(ctr.StartupProbe),
		},
		SecurityContext: containerSecurityStrings(ctr.SecurityContext),
	}
	for _, p := range ctr.Ports {
		cd.Ports = append(cd.Ports, ContainerPort{Name: p.Name, ContainerPort: p.ContainerPort, Protocol: string(p.Protocol)})
	}
	for _, e := range ctr.Env {
		cd.Env = append(cd.Env, EnvVar{Name: e.Name, Value: "[hidden]", Source: envSource(e)})
	}
	for _, ef := range ctr.EnvFrom {
		cd.EnvFrom = append(cd.EnvFrom, envFromString(ef))
	}
	for _, vm := range ctr.VolumeMounts {
		cd.VolumeMounts = append(cd.VolumeMounts, VolumeMount{Name: vm.Name, MountPath: vm.MountPath, ReadOnly: vm.ReadOnly})
	}
	for k, v := range ctr.Resources.Requests {
		cd.Resources.Requests[string(k)] = v.String()
	}
	for k, v := range ctr.Resources.Limits {
		cd.Resources.Limits[string(k)] = v.String()
	}

	for _, st := range statuses {
		if st.Name != ctr.Name {
			continue
		}
		cd.Ready = st.Ready
		cd.RestartCount = st.RestartCount
		switch {
		case st.State.Running != nil:
			cd.State = "Running"
		case st.State.Waiting != nil:
			cd.State = "Waiting: " + st.State.Waiting.Reason
		case st.State.Terminated != nil:
			cd.State = "Terminated: " + st.State.Terminated.Reason
		}
		if t := st.LastTerminationState.Terminated; t != nil {
			cd.LastState = &LastStateInfo{
				Reason:     t.Reason,
				ExitCode:   t.ExitCode,
				Signal:     t.Signal,
				Message:    t.Message,
				StartedAt:  t.StartedAt.Format("2006-01-02 15:04:05"),
				FinishedAt: t.FinishedAt.Format("2006-01-02 15:04:05"),
			}
		}
		break
	}
	return cd
}

// envSource says where a variable's value comes from, never the value itself.
func envSource(e corev1.EnvVar) string {
	vf := e.ValueFrom
	switch {
	case vf == nil:
		return "literal"
	case vf.SecretKeyRef != nil:
		return fmt.Sprintf("secret %s/%s", vf.SecretKeyRef.Name, vf.SecretKeyRef.Key)
	case vf.ConfigMapKeyRef != nil:
		return fmt.Sprintf("configMap %s/%s", vf.ConfigMapKeyRef.Name, vf.ConfigMapKeyRef.Key)
	case vf.FieldRef != nil:
		return "field " + vf.FieldRef.FieldPath
	case vf.ResourceFieldRef != nil:
		return "resource " + vf.ResourceFieldRef.Resource
	}
	return "other"
}

func envFromString(ef corev1.EnvFromSource) string {
	s := "unknown"
	if ef.ConfigMapRef != nil {
		s = "configMap " + ef.ConfigMapRef.Name
	} else if ef.SecretRef != nil {
		s = "secret " + ef.SecretRef.Name
	}
	if ef.Prefix != "" {
		s += " (prefix " + ef.Prefix + ")"
	}
	return s
}

// probeString renders a probe like kubectl describe:
// "http-get :8080/healthz delay=10s timeout=1s period=10s #success=1 #failure=3".
func probeString(p *corev1.Probe) string {
	if p == nil {
		return ""
	}
	var h string
	switch {
	case p.HTTPGet != nil:
		h = fmt.Sprintf("http-get %s://:%s%s", strings.ToLower(orDefault(string(p.HTTPGet.Scheme), "HTTP")), p.HTTPGet.Port.String(), p.HTTPGet.Path)
	case p.TCPSocket != nil:
		h = "tcp-socket :" + p.TCPSocket.Port.String()
	case p.GRPC != nil:
		h = fmt.Sprintf("grpc :%d", p.GRPC.Port)
	case p.Exec != nil:
		h = "exec [" + strings.Join(p.Exec.Command, " ") + "]"
	default:
		h = "unknown"
	}
	return fmt.Sprintf("%s delay=%ds timeout=%ds period=%ds #success=%d #failure=%d",
		h, p.InitialDelaySeconds, p.TimeoutSeconds, p.PeriodSeconds, p.SuccessThreshold, p.FailureThreshold)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func containerSecurityStrings(sc *corev1.SecurityContext) []string {
	if sc == nil {
		return nil
	}
	var out []string
	if sc.Privileged != nil && *sc.Privileged {
		out = append(out, "privileged")
	}
	if sc.RunAsUser != nil {
		out = append(out, fmt.Sprintf("runAsUser=%d", *sc.RunAsUser))
	}
	if sc.RunAsGroup != nil {
		out = append(out, fmt.Sprintf("runAsGroup=%d", *sc.RunAsGroup))
	}
	if sc.RunAsNonRoot != nil {
		out = append(out, fmt.Sprintf("runAsNonRoot=%t", *sc.RunAsNonRoot))
	}
	if sc.ReadOnlyRootFilesystem != nil {
		out = append(out, fmt.Sprintf("readOnlyRootFilesystem=%t", *sc.ReadOnlyRootFilesystem))
	}
	if sc.AllowPrivilegeEscalation != nil {
		out = append(out, fmt.Sprintf("allowPrivilegeEscalation=%t", *sc.AllowPrivilegeEscalation))
	}
	if c := sc.Capabilities; c != nil {
		if len(c.Add) > 0 {
			out = append(out, "capabilities add="+capsString(c.Add))
		}
		if len(c.Drop) > 0 {
			out = append(out, "capabilities drop="+capsString(c.Drop))
		}
	}
	if sc.SeccompProfile != nil {
		out = append(out, "seccomp="+string(sc.SeccompProfile.Type))
	}
	return out
}

func podSecurityStrings(sc *corev1.PodSecurityContext) []string {
	if sc == nil {
		return nil
	}
	var out []string
	if sc.RunAsUser != nil {
		out = append(out, fmt.Sprintf("runAsUser=%d", *sc.RunAsUser))
	}
	if sc.RunAsGroup != nil {
		out = append(out, fmt.Sprintf("runAsGroup=%d", *sc.RunAsGroup))
	}
	if sc.RunAsNonRoot != nil {
		out = append(out, fmt.Sprintf("runAsNonRoot=%t", *sc.RunAsNonRoot))
	}
	if sc.FSGroup != nil {
		out = append(out, fmt.Sprintf("fsGroup=%d", *sc.FSGroup))
	}
	if sc.SeccompProfile != nil {
		out = append(out, "seccomp="+string(sc.SeccompProfile.Type))
	}
	return out
}

func capsString(caps []corev1.Capability) string {
	s := make([]string, len(caps))
	for i, c := range caps {
		s[i] = string(c)
	}
	return strings.Join(s, ",")
}

// tolerationString renders a toleration like "node-role.kubernetes.io/control-plane:NoSchedule op=Exists".
func tolerationString(t corev1.Toleration) string {
	s := orDefault(t.Key, "*")
	if t.Value != "" {
		s += "=" + t.Value
	}
	if t.Effect != "" {
		s += ":" + string(t.Effect)
	}
	s += " op=" + orDefault(string(t.Operator), "Equal")
	if t.TolerationSeconds != nil {
		s += fmt.Sprintf(" for %ds", *t.TolerationSeconds)
	}
	return s
}

// affinityStrings gives one line per affinity term.
func affinityStrings(a *corev1.Affinity) []string {
	if a == nil {
		return nil
	}
	var out []string
	if na := a.NodeAffinity; na != nil {
		if r := na.RequiredDuringSchedulingIgnoredDuringExecution; r != nil {
			for _, term := range r.NodeSelectorTerms {
				out = append(out, "node affinity (required): "+nodeExprs(term.MatchExpressions))
			}
		}
		for _, p := range na.PreferredDuringSchedulingIgnoredDuringExecution {
			out = append(out, fmt.Sprintf("node affinity (preferred, weight %d): %s", p.Weight, nodeExprs(p.Preference.MatchExpressions)))
		}
	}
	podTerms := func(kind string, req []corev1.PodAffinityTerm, pref []corev1.WeightedPodAffinityTerm) {
		for _, t := range req {
			out = append(out, fmt.Sprintf("%s (required): %s on %s", kind, labelSelectorString(t), t.TopologyKey))
		}
		for _, w := range pref {
			out = append(out, fmt.Sprintf("%s (preferred, weight %d): %s on %s", kind, w.Weight, labelSelectorString(w.PodAffinityTerm), w.PodAffinityTerm.TopologyKey))
		}
	}
	if pa := a.PodAffinity; pa != nil {
		podTerms("pod affinity", pa.RequiredDuringSchedulingIgnoredDuringExecution, pa.PreferredDuringSchedulingIgnoredDuringExecution)
	}
	if paa := a.PodAntiAffinity; paa != nil {
		podTerms("pod anti-affinity", paa.RequiredDuringSchedulingIgnoredDuringExecution, paa.PreferredDuringSchedulingIgnoredDuringExecution)
	}
	return out
}

func nodeExprs(exprs []corev1.NodeSelectorRequirement) string {
	s := make([]string, len(exprs))
	for i, e := range exprs {
		s[i] = fmt.Sprintf("%s %s [%s]", e.Key, e.Operator, strings.Join(e.Values, ","))
	}
	return strings.Join(s, ", ")
}

func labelSelectorString(t corev1.PodAffinityTerm) string {
	if t.LabelSelector == nil {
		return "all pods"
	}
	var s []string
	for k, v := range t.LabelSelector.MatchLabels {
		s = append(s, k+"="+v)
	}
	sort.Strings(s)
	for _, e := range t.LabelSelector.MatchExpressions {
		s = append(s, fmt.Sprintf("%s %s [%s]", e.Key, e.Operator, strings.Join(e.Values, ",")))
	}
	return strings.Join(s, ", ")
}
