/*
Copyright 2017 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	"path"
)

// Get a clientset with in-cluster config.
func getClient() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatal(err)
	}
	return clientset
}

func getApiServerCert(clientset *kubernetes.Clientset) []byte {
	c, err := clientset.CoreV1().ConfigMaps("kube-system").Get("extension-apiserver-authentication", metav1.GetOptions{})
	if err != nil {
		glog.Fatal(err)
	}

	pem, ok := c.Data["requestheader-client-ca-file"]
	if !ok {
		glog.Fatalf(fmt.Sprintf("cannot find the ca.crt in the configmap, configMap.Data is %#v", c.Data))
	}
	glog.Info("client-ca-file=", pem)
	return []byte(pem)
}

func configTLS(config Config, clientset *kubernetes.Clientset) *tls.Config {
	cert := getApiServerCert(clientset)
	var err error
	apiserverCA := x509.NewCertPool()
	apiserverCA.AppendCertsFromPEM(cert)


	certPath := path.Join(config.CertDirectory, config.PairName + ".crt")
	glog.Infof("Reading %s", certPath)
	certFile, err := ioutil.ReadFile(certPath)
	if err != nil {
		glog.Fatal(err)
	}

	keyPath := path.Join(config.CertDirectory, config.PairName + ".key")
	glog.Infof("Reading %s", keyPath)
	keyFile, err := ioutil.ReadFile(keyPath)
	if err != nil {
		glog.Fatal(err)
	}
	_, err = certutil.CanReadCertAndKey(string(certFile), string(keyFile))
	if err != nil {
		glog.Fatal("Cannot verify server certificate and key")
	}


	sCert, err := tls.X509KeyPair(certFile, keyFile)
	if err != nil {
		glog.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
		ClientCAs:    apiserverCA,
		// ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}