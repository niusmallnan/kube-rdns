package rdns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/niusmallnan/kube-rdns/controller/k8s"
	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/niusmallnan/rdns-server/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

const (
	contentType     = "Content-Type"
	jsonContentType = "application/json"
)

func jsonBody(payload interface{}) (io.Reader, error) {
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(payload)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

type Client struct {
	httpClient *http.Client
	kubeClient *kubernetes.Clientset
	base       string
}

func (c *Client) request(method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add(contentType, jsonContentType)

	return req, nil
}

func (c *Client) do(req *http.Request) (model.Response, error) {
	var data model.Response
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return data, err
	}
	// when err is nil, resp contains a non-nil resp.Body which must be closed
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, errors.Wrap(err, "Read response body error")
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, errors.Wrap(err, "Decode response error")
	}
	logrus.Infof("Got response entry: %+v", data)
	if code := resp.StatusCode; code < 200 || code > 300 {
		if data.Message != "" {
			return data, errors.Errorf("Got request error: %s", data.Message)
		}
	}

	return data, nil
}

func (c *Client) ApplyDomain(hosts []string) error {
	if len(hosts) == 0 {
		return errors.New("Hosts should not be empty")
	}
	token, fqdn := k8s.GetTokenAndRootFqdn(c.kubeClient)
	if fqdn == "" || token == "" {
		logrus.Debugf("Fqdn for %s has not been exist, need to create a new one", hosts)
		return c.createDomain(hosts)

	}
	d, err := c.getDomain(fqdn)
	if err != nil {
		return err
	}

	sort.Strings(d.Hosts)
	sort.Strings(hosts)
	if !reflect.DeepEqual(d.Hosts, hosts) {
		logrus.Debugf("Fqdn %s has some changes, need to update", fqdn)
		return c.updateDomain(token, fqdn, hosts)
	}
	logrus.Debugf("Fqdn %s has no changes, no need to update", fqdn)

	return nil
}

func (c *Client) getDomain(fqdn string) (d model.Domain, err error) {
	url := fmt.Sprintf("%s/domain/%s", c.base, fqdn)
	req, err := c.request(http.MethodGet, url, nil)
	if err != nil {
		return d, errors.Wrap(err, "getDomain: failed to build a request")
	}

	o, err := c.do(req)
	if err != nil {
		return d, errors.Wrap(err, "getDomain: failed to execute a request")
	}

	return o.Data, nil
}

func (c *Client) createDomain(hosts []string) error {
	url := fmt.Sprintf("%s/domain", c.base)
	body, err := jsonBody(&model.DomainOptions{Hosts: hosts})
	if err != nil {
		return err
	}

	req, err := c.request(http.MethodPost, url, body)
	if err != nil {
		return errors.Wrap(err, "createDomain: failed to build a request")
	}

	rep, err := c.do(req)
	if err != nil {
		return errors.Wrap(err, "createDomain: failed to execute a request")
	}

	k8s.SaveTokenAndRootFqdn(c.kubeClient, rep.Token, rep.Data.Fqdn)

	return err
}

func (c *Client) updateDomain(token, fqdn string, hosts []string) error {
	url := fmt.Sprintf("%s/domain/%s", c.base, fqdn)
	body, err := jsonBody(&model.DomainOptions{Hosts: hosts})
	if err != nil {
		return err
	}

	req, err := c.request(http.MethodPut, url, body)
	if err != nil {
		return errors.Wrap(err, "updateDomain: failed to build a request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	_, err = c.do(req)
	if err != nil {
		return errors.Wrap(err, "updateDomain: failed to execute a request")
	}

	return err
}

func (c *Client) RenewDomain() error {
	token, fqdn := k8s.GetTokenAndRootFqdn(c.kubeClient)
	if token == "" || fqdn == "" {
		return errors.New("RenewDomain: failed to get token and fqdn")
	}

	url := fmt.Sprintf("%s/domain/%s/renew", c.base, fqdn)

	req, err := c.request(http.MethodPut, url, nil)
	if err != nil {
		return errors.Wrap(err, "RenewDomain: failed to build a request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	_, err = c.do(req)
	if err != nil {
		return errors.Wrap(err, "RenewDomain: failed to execute a request")
	}

	return err
}

func NewClient(kubeClient *kubernetes.Clientset) *Client {
	httpClient := &http.Client{Timeout: 5 * time.Second}
	return &Client{
		httpClient: httpClient,
		kubeClient: kubeClient,
		base:       setting.GetBaseRdnsURL(),
	}
}
