/*
Copyright (c) 2019, VMware
All rights reserved.
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name VMware nor the
      names of its contributors may be used to endorse or promote products
      derived from this software without specific prior written permission.
THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL VMWARE BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOW CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
*/

package command

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

func Run(cmd *exec.Cmd) {
	errPipe, err := cmd.StderrPipe()
	gomega.ExpectWithOffset(1, err).To(gomega.BeNil())

	// Log output
	go captureOutput(errPipe, "stderr")
	if cmd.Stdout == nil {
		outPipe, err := cmd.StdoutPipe()
		gomega.ExpectWithOffset(1, err).To(gomega.BeNil())
		go captureOutput(outPipe, "stdout")
	}
	gomega.ExpectWithOffset(1, cmd.Run()).To(gomega.BeNil())
}

func captureOutput(pipe io.ReadCloser, label string) {
	buffer := &bytes.Buffer{}
	defer pipe.Close()
	for {
		n, _ := buffer.ReadFrom(pipe)
		if n == 0 {
			return
		}
		fmt.Fprintf(ginkgo.GinkgoWriter, "[%s] %s\n", label, buffer.String())
	}
}
