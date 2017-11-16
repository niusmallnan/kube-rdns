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

	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/niusmallnan/rdns-server/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

func (c *Client) ApplyDomain(fqdn string, hosts []string) error {
	d, err := c.GetDomain(fqdn)
	if err != nil {
		return err
	}

	if d.Fqdn == "" {
		logrus.Debugf("Fqdn %s has not been exist, need to create a new one", fqdn)
		return c.CreateDomain(fqdn, hosts)
	}

	sort.Strings(d.Hosts)
	sort.Strings(hosts)
	if !reflect.DeepEqual(d.Hosts, hosts) {
		logrus.Debugf("Fqdn %s has been exist, need to update", fqdn)
		return c.UpdateDomain(fqdn, hosts)
	}

	return nil
}

func (c *Client) GetDomain(fqdn string) (d model.Domain, err error) {
	url := fmt.Sprintf("%s/domain/%s", c.base, fqdn)
	req, err := c.request(http.MethodGet, url, nil)
	if err != nil {
		return d, errors.Wrap(err, "GetDomain: failed to build a request")
	}

	o, err := c.do(req)
	if err != nil {
		return d, errors.Wrap(err, "GetDomain: failed to execute a request")
	}

	return o.Data, nil
}

func (c *Client) CreateDomain(fqdn string, hosts []string) error {
	url := fmt.Sprintf("%s/domain", c.base)
	body, err := jsonBody(&model.DomainOptions{Fqdn: fqdn, Hosts: hosts})
	if err != nil {
		return err
	}

	req, err := c.request(http.MethodPost, url, body)
	if err != nil {
		return errors.Wrap(err, "CreateDomain: failed to build a request")
	}

	_, err = c.do(req)
	if err != nil {
		return errors.Wrap(err, "CreateDomain: failed to execute a request")
	}

	return err
}

func (c *Client) UpdateDomain(fqdn string, hosts []string) error {
	url := fmt.Sprintf("%s/domain/%s", c.base, fqdn)
	body, err := jsonBody(&model.DomainOptions{Hosts: hosts})
	if err != nil {
		return err
	}

	req, err := c.request(http.MethodPut, url, body)
	if err != nil {
		return errors.Wrap(err, "UpdateDomain: failed to build a request")
	}

	_, err = c.do(req)
	if err != nil {
		return errors.Wrap(err, "UpdateDomain: failed to execute a request")
	}

	return err
}

func (c *Client) DeleteDomain(fqdn string) error {
	url := fmt.Sprintf("%s/domain/%s", c.base, fqdn)

	req, err := c.request(http.MethodDelete, url, nil)
	if err != nil {
		return errors.Wrap(err, "DeleteDomain: failed to build a request")
	}

	_, err = c.do(req)
	if err != nil {
		return errors.Wrap(err, "DeleteDomain: failed to execute a request")
	}

	return err
}

func (c *Client) RenewDomain(fqdn string) error {
	url := fmt.Sprintf("%s/domain/%s/renew", c.base, fqdn)

	req, err := c.request(http.MethodPut, url, nil)
	if err != nil {
		return errors.Wrap(err, "RenewDomain: failed to build a request")
	}

	_, err = c.do(req)
	if err != nil {
		return errors.Wrap(err, "RenewDomain: failed to execute a request")
	}

	return err
}

func NewClient() *Client {
	return &Client{httpClient: http.DefaultClient,
		base: setting.GetBaseRdnsURL()}
}
