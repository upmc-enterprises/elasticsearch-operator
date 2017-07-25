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

package elastic

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var (
	healthEndpoint = "/_cluster/health"
)

// StatusEvent describes cluster status events
type StatusEvent struct {
	Type string
	HealthStatus
}

// MonitorElasticClusterStatus watches for elastic events
func (c *Client) MonitorElasticClusterStatus(stopchan chan struct{}) (<-chan *StatusEvent, <-chan error) {
	events := make(chan *StatusEvent)
	errc := make(chan error, 1)
	go func() {
		for {
			// Load CA cert
			caCert, err := ioutil.ReadFile(caFile)
			if err != nil {
				log.Fatal(err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)

			// Setup HTTPS client
			tlsConfig := &tls.Config{
				RootCAs: caCertPool,
			}
			tlsConfig.BuildNameToCertificate()
			tlsConfig.InsecureSkipVerify = true
			transport := &http.Transport{TLSClientConfig: tlsConfig}

			httpClient := &http.Client{Transport: transport}

			resp, err := httpClient.Get(c.elasticURL + healthEndpoint)
			if err != nil {
				errc <- err
				time.Sleep(5 * time.Second)
				continue
			}

			if resp.StatusCode != 200 {
				errc <- errors.New("Invalid status code: " + resp.Status)
				time.Sleep(5 * time.Second)
				continue
			} else {
				decoder := json.NewDecoder(resp.Body)
				var event StatusEvent
				err = decoder.Decode(&event)

				// Is uninitialized?
				if c.HealthStatus == (HealthStatus{}) {
					// There was no initial status, broadcast out the current
					events <- &event
				} else {
					if c.HealthStatus.Status != event.HealthStatus.Status {
						// Status has changed
						events <- &event
					}
				}

				// Always update the local cache with current version
				c.HealthStatus = event.HealthStatus
			}

			// for {
			// 	var event StatusEvent
			// 	err = decoder.Decode(&event)
			// 	if err != nil {
			// 		errc <- err
			// 		break
			// 	}
			// 	events <- &event
			// }

			// Zzz...zzzzzZZzzz...zzzZZZzzzz...
			time.Sleep(5 * time.Second)
		}
	}()

	return events, errc
}
