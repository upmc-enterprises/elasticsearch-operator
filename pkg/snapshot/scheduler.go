/*
Copyright (c) 2017, UPMC Enterprises
All rights reserved.
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name UPMC Enterprises nor the
      names of its contributors may be used to endorse or promote products
      derived from this software without specific prior written permission.
THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL UPMC ENTERPRISES BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
*/

package snapshot

import (
	"fmt"
	"net/http"

	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	enterprisesv1 "github.com/upmc-enterprises/elasticsearch-operator/pkg/apis/elasticsearchoperator/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1beta1 "k8s.io/api/batch/v1beta1"
	apicore "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	defaultCronImage     = "upmcenterprises/elasticsearch-cron:0.0.4"
	cronActionRepository = "create-repository"
	cronActionSnapshot   = "snapshot"
)

type Scheduler struct {
	Kclient kubernetes.Interface
	CRD     enterprisesv1.Scheduler
}

// New creates an instance of Scheduler
func New(repoType, bucketName, cronSchedule string, enabled, useSSL bool, userName, password, image,
	elasticURL, clusterName, namespace, repoAccessKey, repoSecretKey, repoRegion string, kc kubernetes.Interface) *Scheduler {

	if repoType == "" {
		repoType = "s3"
	}

	if image == "" {
		image = defaultCronImage
	}

	return &Scheduler{
		Kclient: kc,
		CRD: enterprisesv1.Scheduler{
			RepoType:     repoType,
			BucketName:   bucketName,
			CronSchedule: cronSchedule,
			ElasticURL:   elasticURL,
			Auth: enterprisesv1.SchedulerAuthentication{
				UserName: userName,
				Password: password,
			},
			RepoAuth: enterprisesv1.RepoSchedulerAuthentication{
				RepoAccessKey: repoAccessKey,
				RepoSecretKey: repoSecretKey,
			},
			RepoRegion:  repoRegion,
			UseSSL:      useSSL,
			Namespace:   namespace,
			ClusterName: clusterName,
			Enabled:     enabled,
			Image:       image,
		},
	}
}

// Init creates the snapshot repository cronjob
func (s *Scheduler) Init() error {

	if s.CRD.Enabled {
		// Init repository
		if err := s.CreateSnapshotRepository(); err != nil {
			return err
		}

		// Init snapshot
		if err := s.CreateSnapshot(); err != nil {
			return err
		}
	}
	return nil
}

// CreateSnapshotRepository creates the snapshot repository cronjob
func (s *Scheduler) CreateSnapshotRepository() error {
	// TODO: This should wait until the api goes green and cluster is healthy
	return s.CreateCronJob(s.CRD.Namespace, s.CRD.ClusterName, cronActionRepository, s.CRD.CronSchedule)
}

// CreateSnapshot creates snapshot cronjob
func (s *Scheduler) CreateSnapshot() error {
	return s.CreateCronJob(s.CRD.Namespace, s.CRD.ClusterName, cronActionSnapshot, s.CRD.CronSchedule)
}

// Stop cleans up Cron
func (s *Scheduler) Stop() {
	s.deleteCronJob(s.CRD.Namespace, s.CRD.ClusterName)
	s.deleteJobs(s.CRD.Namespace, s.CRD.ClusterName)
}

// DeleteJobs cleans up an remaining jobs started by the cronjob
func (s *Scheduler) deleteJobs(namespace, clusterName string) {
	err := s.Kclient.BatchV1().Jobs(namespace).DeleteCollection(
		&metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=elasticsearch-operator,clusterName=%s", clusterName),
		})

	// ignore not found error
	if err != nil {
		if err.(*apierrors.StatusError).ErrStatus.Code != http.StatusNotFound {
			logrus.Error("Could not delete Jobs! ", err)
		}
	}

}

// DeleteCronJob deletes a cron job
func (s *Scheduler) deleteCronJob(namespace, clusterName string) {
	// Repository CronJob
	snapshotName := getSnapshotname(clusterName, cronActionRepository)
	err := s.Kclient.BatchV1beta1().CronJobs(namespace).Delete(snapshotName, &metav1.DeleteOptions{})

	// ignore not found error
	if err != nil {
		if _, ok := err.(*apierrors.StatusError); ok {
			if err.(*apierrors.StatusError).ErrStatus.Code != http.StatusNotFound {
				logrus.Error("Could not delete Repository CronJob! ", err)
			}
		}
	}

	// Snapshot CronJob
	snapshotName = getSnapshotname(clusterName, cronActionSnapshot)
	err = s.Kclient.BatchV1beta1().CronJobs(namespace).Delete(snapshotName, &metav1.DeleteOptions{})

	// ignore not found error
	if err != nil {
		if _, ok := err.(*apierrors.StatusError); ok {
			if err.(*apierrors.StatusError).ErrStatus.Code != http.StatusNotFound {
				logrus.Error("Could not delete CronJob! ", err)
			}
		}
	}

}

// CreateCronJob creates a cron job
func (s *Scheduler) CreateCronJob(namespace, clusterName, action, cronSchedule string) error {
	snapshotName := getSnapshotname(clusterName, action)

	// Check if CronJob exists
	cronJob, err := s.Kclient.BatchV1beta1().CronJobs(namespace).Get(snapshotName, metav1.GetOptions{})

	if len(cronJob.Name) == 0 {

		requestCPU, err := resource.ParseQuantity("100m")
		if err != nil {
			return err
		}

		requestMemory, err := resource.ParseQuantity("256mbi")
		if err == nil {
			return err
		}
		job := &v1beta1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name: snapshotName,
				Labels: map[string]string{
					"app":         "elasticsearch-operator",
					"clusterName": clusterName,
					"name":        snapshotName,
				},
			},
			Spec: v1beta1.CronJobSpec{
				Schedule: cronSchedule,
				JobTemplate: v1beta1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: apicore.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app":         "elasticsearch-operator",
									"name":        snapshotName,
									"clusterName": clusterName,
								},
							},
							Spec: apicore.PodSpec{
								RestartPolicy: "OnFailure",
								Containers: []apicore.Container{
									apicore.Container{
										Name:            snapshotName,
										Image:           s.CRD.Image,
										ImagePullPolicy: "Always",
										Resources: apicore.ResourceRequirements{
											Requests: apicore.ResourceList{
												"cpu":    requestCPU,
												"memory": requestMemory,
											},
										},
										Args: []string{
											fmt.Sprintf("--action=%s", action),
											fmt.Sprintf("--repo-type=%s", s.CRD.RepoType),
											fmt.Sprintf("--bucket-name=%s", s.CRD.BucketName),
											fmt.Sprintf("--elastic-url=%s", s.CRD.ElasticURL),
											fmt.Sprintf("--auth-username=%s", s.CRD.Auth.UserName),
											fmt.Sprintf("--auth-password=%s", s.CRD.Auth.Password),
											fmt.Sprintf("--repo-auth-access-key=%s", s.CRD.RepoAuth.RepoAccessKey),
											fmt.Sprintf("--repo-auth-secret-key=%s", s.CRD.RepoAuth.RepoSecretKey),
											fmt.Sprintf("--repo-region=%s", s.CRD.RepoRegion),
											fmt.Sprintf("--use-ssl=%t", s.CRD.UseSSL),
										},
									},
								},
							},
						},
					},
				},
			},
		}

		if _, err := s.Kclient.BatchV1beta1().CronJobs(namespace).Create(job); err != nil {
			logrus.Error("Could not create CronJob! ", err)
			return err
		}
	} else if err != nil {
		logrus.Error("Could not get cron job! ", err)
		return err
	}
	logrus.Infof("CronJob %v succesfully created ! ", snapshotName)

	return nil
}

// GetSnapshotname gets the name of the snapshot cron job
func getSnapshotname(clusterName, action string) string {
	return fmt.Sprintf("elastic-%s-%s", clusterName, action)
}
