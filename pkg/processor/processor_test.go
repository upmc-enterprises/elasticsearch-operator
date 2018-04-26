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

package processor

import (
	"testing"
)

func TestBaseImage(t *testing.T) {
	expectedImage := "foo/image"
	p, _ := New(nil, expectedImage)

	if p.baseImage != expectedImage {
		t.Error("Expected "+expectedImage+", got ", p.baseImage)
	}

}

func TestCustomImage(t *testing.T) {
	expectedImage := "custom/image:tag"
	p, _ := New(nil, "foo/image")

	p.baseImage = p.calcBaseImage(p.baseImage, expectedImage)

	if p.baseImage != expectedImage {
		t.Error("Expected "+expectedImage+", got ", p.baseImage)
	}

}

func TestZones1(t *testing.T) {
	processor, _ := New(nil, "foo/image")

	zoneDistribution := processor.calculateZoneDistribution(5, 3)

	if zoneDistribution[0] != 2 {
		t.Error("Expected 2, got ", zoneDistribution[0])
	}

	if zoneDistribution[1] != 2 {
		t.Error("Expected 2, got ", zoneDistribution[0])
	}

	if zoneDistribution[2] != 1 {
		t.Error("Expected 1, got ", zoneDistribution[0])
	}

	if len(zoneDistribution) != 3 {
		t.Error("Expected 3 zones, got ", len(zoneDistribution))
	}
}

func TestZones2(t *testing.T) {
	processor, _ := New(nil, "foo/image")

	zoneDistribution := processor.calculateZoneDistribution(5, 2)

	if zoneDistribution[0] != 3 {
		t.Error("Expected 3, got ", zoneDistribution[0])
	}

	if zoneDistribution[1] != 2 {
		t.Error("Expected 2, got ", zoneDistribution[0])
	}

	if len(zoneDistribution) != 2 {
		t.Error("Expected 2 zones, got ", len(zoneDistribution))
	}
}

func TestZones3(t *testing.T) {
	processor, _ := New(nil, "foo/image")

	zoneDistribution := processor.calculateZoneDistribution(5, 1)

	if zoneDistribution[0] != 5 {
		t.Error("Expected 5, got ", zoneDistribution[0])
	}
}

func TestNoZonesPassed(t *testing.T) {
	processor, _ := New(nil, "foo/image")

	zoneDistribution := processor.calculateZoneDistribution(5, 0)

	if zoneDistribution[0] != 5 {
		t.Error("Expected 5, got ", zoneDistribution[0])
	}

	if len(zoneDistribution) != 1 {
		t.Error("Expected 1 zone, got ", len(zoneDistribution))
	}
}

func TestDefaultUseSSL(t *testing.T) {
	processor, _ := New(nil, "foo/image")

	useSSL := processor.defaultUseSSL(nil)
	if useSSL != true {
		t.Errorf("Expected useSSL to default to true when not specified, got %v", useSSL)
	}

	useSSL = true
	useSSL = processor.defaultUseSSL(&useSSL)
	if useSSL != true {
		t.Errorf("Expected useSSL to be true when specified as true, got %v", useSSL)
	}

	useSSL = false
	useSSL = processor.defaultUseSSL(&useSSL)
	if useSSL != false {
		t.Errorf("Expected useSSL to be false when specified as false, got %v", useSSL)
	}
}
