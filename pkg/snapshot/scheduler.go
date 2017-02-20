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
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	cron "github.com/robfig/cron"
)

var (
	elasticURL = fmt.Sprintf("https://%s:9200/", "elasticsearch") // Internal service name of cluster
)

// Scheduler stores info about how to snapshot the cluster
type Scheduler struct {
	s3bucketName string
	cronSchedule string
	enabled      bool
	cron         *cron.Cron
}

// New creates an instance of Scheduler
func New(bucketName, cronSchedule string, enabled bool) *Scheduler {
	return &Scheduler{
		s3bucketName: bucketName,
		cronSchedule: cronSchedule,
		cron:         cron.New(),
		enabled:      enabled,
	}
}

// Run starts the automated scheduler
func (s *Scheduler) Run() {
	if s.enabled {
		logrus.Info("-----> Init scheduler")

		logrus.Infof("Cron is set to %s", s.cronSchedule)
		s.cron.AddFunc(s.cronSchedule, func() {
			s.CreateSnapshotRepository()
			s.CreateSnapshot()
		})
		s.cron.Start()
	} else {
		logrus.Info("Scheduler is disabled, no snapshots will be scheduled")
	}
}

// CreateSnapshotRepository creates a repository to place snapshots
func (s *Scheduler) CreateSnapshotRepository() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	body := fmt.Sprintf("{ \"type\": \"s3\", \"settings\": { \"bucket\": \"%s\" } }", s.s3bucketName)
	url := fmt.Sprintf("%s_snapshot/%s", elasticURL, s.s3bucketName)
	req, err := http.NewRequest("PUT", url, strings.NewReader(body))
	resp, err := client.Do(req)

	// Some other type of error?
	if err != nil {
		logrus.Error("Error attempting to create snapshot repository: ", err)
		return
	}

	// Non 2XX status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logrus.Errorf("Error creating snapshot repository [httpstatus: %d][url: %s]", resp.StatusCode, url)
		return
	}

	logrus.Infof("Created snapshot repository!")

	return
}

// CreateSnapshot makes a snapshot of all indexes
func (s *Scheduler) CreateSnapshot() {
	logrus.Info("About to create snapshot...")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	url := fmt.Sprintf("%s_snapshot/%s/snapshot_%s", elasticURL, s.s3bucketName, fmt.Sprintf(time.Now().Format("2006-01-02-15-04-05")))

	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		logrus.Error("Error attempting to create snapshot: ", err)
		return
	}

	resp, err := client.Do(req)

	// Some other type of error?
	if err != nil {
		logrus.Error("Error attempting to create snapshot: ", err)
		return
	}

	// Non 2XX status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logrus.Errorf("Error creating snapshot [httpstatus: %d][url: %s]", resp.StatusCode, url)
		return
	}

	logrus.Infof("Created snapshot!")

	return
}
