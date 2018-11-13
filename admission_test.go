package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	"path/filepath"
	"testing"
)

var admitTests = []struct {
	file    string
	mode    string
	allowed bool
}{
	{"ar_missing_affinity.json", "denyMissing", false},
	{"ar_with_affinity.json", "denyMissing", true},
	{"ar_missing_affinity.json", "patchAlways", true},
	{"ar_with_affinity.json", "patchAlways", true},
	{"ar_missing_affinity.json", "patchMissing", true},
	{"ar_with_affinity.json", "patchMissing", true},
}

func TestAdmit(t *testing.T) {
	for _, tt := range admitTests {
		t.Run(tt.file, func(t *testing.T) {
			admissionReview := loadFixture(tt.file, t)
			admissionResponse := admitPods(admissionReview, &Config{Mode: tt.mode})
			if admissionResponse.Allowed != tt.allowed {
				t.Errorf("In %s mode for %s got %v, wanted %v", tt.mode, tt.file, admissionResponse.Allowed, tt.allowed)
			}
		})
	}
}

var mutatePatchExpectation = `[{"op":"add","path":"/spec/affinity","value":{}}]`
var mutateTests = []struct {
	file  string
	mode  string
	patch []byte
}{
	{"ar_missing_affinity.json", "denyMissing", nil},
	{"ar_with_affinity.json", "denyMissing", nil},
	{"ar_missing_affinity.json", "patchAlways", []byte(mutatePatchExpectation)},
	{"ar_with_affinity.json", "patchAlways", []byte(mutatePatchExpectation)},
	{"ar_missing_affinity.json", "patchMissing", []byte(mutatePatchExpectation)},
	{"ar_with_affinity.json", "patchMissing", nil},
}

func TestMutate(t *testing.T) {
	for _, tt := range mutateTests {
		t.Run(tt.file, func(t *testing.T) {
			admissionReview := loadFixture(tt.file, t)
			admissionResponse := mutatePods(admissionReview, &Config{Mode: tt.mode, AffinityPatch: "{}"})
			if bytes.Compare(admissionResponse.Patch, tt.patch) != 0 {
				t.Errorf("In %s mode for %s got %s, wanted %s", tt.mode, tt.file, admissionResponse.Patch, tt.patch)
			}
		})
	}
}

var mutatePatchWithSelectorExpectation = `[{"op":"add","path":"/spec/affinity","value":{}}]`
var mutatePatchWithSelectorTests = []struct {
	file     string
	mode     string
	selector string
	patch    []byte
}{
	{"ar_missing_affinity.json", "patchMissing", `{}`, []byte(mutatePatchWithSelectorExpectation)},
	{"ar_missing_affinity.json", "patchMissing", `{"matchLabels":{"app":"mysql"}}`, nil},
	{"ar_missing_affinity.json", "patchMissing", `{"matchLabels":{"app":"nginx"}}`, []byte(mutatePatchWithSelectorExpectation)},
	{"ar_missing_affinity.json", "patchAlways", `{"matchLabels":{"app":"mysql"}}`, nil},
	{"ar_with_affinity.json", "patchAlways", `{"matchLabels":{"app":"mysql"}}`, nil},
	{"ar_missing_affinity.json", "patchAlways", `{"matchLabels":{"app":"nginx"}}`, []byte(mutatePatchExpectation)},
	{"ar_with_affinity.json", "patchAlways", `{"matchLabels":{"app":"nginx"}}`, []byte(mutatePatchExpectation)},
}

func TestMutateWithLabel(t *testing.T) {
	for _, tt := range mutatePatchWithSelectorTests {
		t.Run(tt.file, func(t *testing.T) {
			selector, err := parsePodSelector(tt.selector)
			if err != nil {
				t.Errorf("%v", err)
			}

			admissionReview := loadFixture(tt.file, t)
			admissionResponse := mutatePods(admissionReview, &Config{Mode: tt.mode, PodSelector: selector, AffinityPatch: "{}"})
			if bytes.Compare(admissionResponse.Patch, tt.patch) != 0 {
				t.Errorf("In %s mode for %s got %s", "patchMissing", "ar_missing_affinity.json", admissionResponse.Patch)
			}
		})
	}
}

func loadFixture(file string, t *testing.T) v1beta1.AdmissionReview {
	filename := filepath.Join("testdata", file)
	jsonFixture, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	admissionReview := v1beta1.AdmissionReview{}
	err = json.Unmarshal(jsonFixture, &admissionReview)
	if err != nil {
		t.Fatal(err)
	}
	return admissionReview
}
