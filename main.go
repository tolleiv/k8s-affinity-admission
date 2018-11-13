package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
)

type Config struct {
	PairName      string
	CertDirectory string
	PortNumber    string
	Mode          string
	AffinityPatch string
	PodSelectorS  string
	PodSelector   labels.Selector
}

func (c *Config) addFlags() {
	flag.StringVar(&c.PairName, "keypairname", "tls", "certificate and key pair name")
	flag.StringVar(&c.CertDirectory, "certdir", "/var/run/affinity-admission-controller", "certificate and key directory")
	flag.StringVar(&c.PortNumber, "port", "8443", "webserver port")
	flag.StringVar(&c.Mode, "mode", "patchMissing", "")
	flag.StringVar(&c.AffinityPatch, "affinityPatch", "{}", "")
	flag.StringVar(&c.PodSelectorS, "podSelector", "", "")
}

func admitPods(ar v1beta1.AdmissionReview, config *Config) *v1beta1.AdmissionResponse {
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		err := fmt.Errorf("expect resource to be %s", podResource)
		return toAdmissionResponse(err)
	}
	raw := ar.Request.Object.Raw
	pod := v1.Pod{}
	if err := json.Unmarshal(raw, &pod); err != nil {
		return toAdmissionResponse(err)
	}
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = (config.Mode != "denyMissing" || pod.Spec.Affinity != nil)
	return &reviewResponse
}

func mutatePods(ar v1beta1.AdmissionReview, config *Config) *v1beta1.AdmissionResponse {
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		err := fmt.Errorf("expect resource to be %s", podResource)
		return toAdmissionResponse(err)
	}
	raw := ar.Request.Object.Raw
	pod := v1.Pod{}
	if err := json.Unmarshal(raw, &pod); err != nil {
		return toAdmissionResponse(err)
	}
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true

	if !(config.Mode == "patchAlways" || config.Mode == "patchMissing") {
		glog.V(2).Infof("%s mode - not patching", config.Mode)
		return &reviewResponse
	}
	if config.Mode == "patchMissing" && pod.Spec.Affinity != nil {
		glog.V(2).Infof("affinity found - not patching")
		return &reviewResponse
	}

	if config.PodSelector != nil &&!config.PodSelector.Matches(labels.Set(pod.GetLabels())) {
		glog.V(2).Infof("Pod labels did not match - found %v",pod.GetLabels())
		return &reviewResponse
	}

	affinityPatch := fmt.Sprintf(`[{"op":"add","path":"/spec/affinity","value":%s}]`, config.AffinityPatch)

	glog.V(2).Infof("patching pod")
	reviewResponse.Patch = []byte(affinityPatch)
	pt := v1beta1.PatchTypeJSONPatch
	reviewResponse.PatchType = &pt
	return &reviewResponse
}

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	glog.Error(err)
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

type admitFunc func(v1beta1.AdmissionReview, *Config) *v1beta1.AdmissionResponse

func serve(w http.ResponseWriter, r *http.Request, admit admitFunc, config *Config) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	glog.V(2).Info(fmt.Sprintf("handling request: %v", string(body)))
	var reviewResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Error(err)
		reviewResponse = toAdmissionResponse(err)
	} else {
		reviewResponse = admit(ar, config)
	}
	glog.V(2).Info(fmt.Sprintf("sending response: %v", reviewResponse))

	response := v1beta1.AdmissionReview{}
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = ar.Request.UID
	}
	// reset the Object and OldObject, they are not needed in a response.
	ar.Request.Object = runtime.RawExtension{}
	ar.Request.OldObject = runtime.RawExtension{}

	resp, err := json.Marshal(response)
	if err != nil {
		glog.Error(err)
	}
	if _, err := w.Write(resp); err != nil {
		glog.Error(err)
	}
}

func serveAdmit(config *Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		serve(w, r, admitPods, config)
	}
}

func serveMutate(config *Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		serve(w, r, mutatePods, config)
	}
}

func serveHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func parsePodSelector(podSelectorS string) (labels.Selector, error) {
	labelSelector := &metav1.LabelSelector{}
	err := json.Unmarshal([]byte(podSelectorS), labelSelector)
	if err != nil {
		return nil, err
	}
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, err
	}
	return selector, nil
}

func main() {
	var config Config
	config.addFlags()
	flag.Parse()
	if config.PodSelectorS != "" {
		selector, err := parsePodSelector(config.PodSelectorS)
		if err != nil {
			glog.Fatal(err)
		}
		config.PodSelector = selector
	}

	http.HandleFunc("/admit", serveAdmit(&config))
	http.HandleFunc("/mutate", serveMutate(&config))
	http.HandleFunc("/health", serveHealth)
	clientset := getClient()
	server := &http.Server{
		Addr:      fmt.Sprintf(":%s", config.PortNumber),
		TLSConfig: configTLS(config, clientset),
	}
	glog.V(2).Infof("starting webserver on port %s", config.PortNumber)
	glog.V(2).Infof("mode and affinityPatch: %s=%s", config.Mode, config.AffinityPatch)
	if err := server.ListenAndServeTLS("", ""); err != nil {
		glog.Fatal(err)
	}
}

