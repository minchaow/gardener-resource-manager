// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package health

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

var (
	trueCrdConditionTypes = []apiextensionsv1beta1.CustomResourceDefinitionConditionType{
		apiextensionsv1beta1.NamesAccepted, apiextensionsv1beta1.Established,
	}
	falseOptionalCrdConditionTypes = []apiextensionsv1beta1.CustomResourceDefinitionConditionType{
		apiextensionsv1beta1.Terminating,
	}
)

// CheckCustomResourceDefinition checks whether the given CustomResourceDefinition is healthy.
// A CRD is considered healthy if its `NamesAccepted` and `Established` conditions are with status `True`
// and its `Terminating` condition is missing or has status `False`.
func CheckCustomResourceDefinition(crd *apiextensionsv1beta1.CustomResourceDefinition) error {
	for _, trueConditionType := range trueCrdConditionTypes {
		conditionType := string(trueConditionType)
		condition := getCustomResourceDefinitionCondition(crd.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseOptionalConditionType := range falseOptionalCrdConditionTypes {
		conditionType := string(falseOptionalConditionType)
		condition := getCustomResourceDefinitionCondition(crd.Status.Conditions, falseOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

// CheckJob checks whether the given Job is healthy.
// A Job is considered healthy if its `JobFailed` condition is missing or has status `False`.
func CheckJob(job *batchv1.Job) error {
	condition := getJobCondition(job.Status.Conditions, batchv1.JobFailed)
	if condition == nil {
		return nil
	}
	if err := checkConditionState(string(batchv1.JobFailed), string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
		return err
	}

	return nil
}

var (
	healthyPodPhases = []corev1.PodPhase{
		corev1.PodRunning, corev1.PodSucceeded,
	}
)

// CheckPod checks whether the given Pod is healthy.
// A Pod is considered healthy if its `.status.phase` is `Running` or `Succeeded`.
func CheckPod(pod *corev1.Pod) error {
	var phase = pod.Status.Phase
	for _, healthyPhase := range healthyPodPhases {
		if phase == healthyPhase {
			return nil
		}
	}

	return fmt.Errorf("pod is in invalid phase %q (expected one of %q)",
		phase, healthyPodPhases)
}

// CheckReplicaSet checks whether the given ReplicaSet is healthy.
// A ReplicaSet is considered healthy if the controller observed its current revision and
// if the number of ready replicas is equal to the number of replicas.
func CheckReplicaSet(rs *appsv1.ReplicaSet) error {
	if rs.Status.ObservedGeneration < rs.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", rs.Status.ObservedGeneration, rs.Generation)
	}

	var replicas = rs.Spec.Replicas
	if replicas != nil && rs.Status.ReadyReplicas < *replicas {
		return fmt.Errorf("ReplicaSet does not have minimum availability")
	}

	return nil
}

// CheckReplicationController check whether the given ReplicationController is healthy.
// A ReplicationController is considered healthy if the controller observed its current revision and
// if the number of ready replicas is equal to the number of replicas.
func CheckReplicationController(rc *corev1.ReplicationController) error {
	if rc.Status.ObservedGeneration < rc.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", rc.Status.ObservedGeneration, rc.Generation)
	}

	var replicas = rc.Spec.Replicas
	if replicas != nil && rc.Status.ReadyReplicas < *replicas {
		return fmt.Errorf("ReplicationController does not have minimum availability")
	}

	return nil
}

func getCustomResourceDefinitionCondition(conditions []apiextensionsv1beta1.CustomResourceDefinitionCondition, conditionType apiextensionsv1beta1.CustomResourceDefinitionConditionType) *apiextensionsv1beta1.CustomResourceDefinitionCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

func getJobCondition(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) *batchv1.JobCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

func requiredConditionMissing(conditionType string) error {
	return fmt.Errorf("condition %q is missing", conditionType)
}

func checkConditionState(conditionType string, expected, actual, reason, message string) error {
	if expected != actual {
		return fmt.Errorf("condition %q has invalid status %s (expected %s) due to %s: %s",
			conditionType, actual, expected, reason, message)
	}
	return nil
}