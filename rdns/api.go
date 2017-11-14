package rdns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/niusmallnan/rdns-server/model"
	"github.com/pkg/errors"
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
	c    *http.Client
	base string
}

func (c *Client) request(method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add(contentType, jsonContentType)

	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return resp, err
	}
	// when err is nil, resp contains a non-nil resp.Body which must be closed
	defer resp.Body.Close()

	if code := resp.StatusCode; code < 200 || code > 300 {
		var dat map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(dat)
		if err != nil {
			return resp, err
		}
		if msg, ok := data["msg"].(string); ok && msg != "" {
			return resp, errors.Errorf("Got request error: %s", msg)
		}
	}

	return resp, nil
}

func (c *Client) ApplyDomain(fqdn string, hosts []string) error {
	exist, err := c.ExistDomain(fqdn)
	if err != nil {
		return err
	}

	if exist {
		return c.UpdateDomain(fqdn, hosts)
	}

	return c.CreateDomain(fqdn, hosts)
}

func (c *Client) ExistDomain(fqdn string) (bool, error) {
	url := fmt.Sprintf("%s/domain/%s", c.base, fqdn)
	req, err := c.request(http.MethodDelete, url, nil)
	if err != nil {
		return false, errors.Wrap(err, "GetDomain: failed to build a request")
	}

	_, err := c.do(req)
	if err != nil {
		return false, errors.Wrap(err, "GetDomain: failed to execute a request")
	}

	return true, nil
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

	_, err := c.do(req)
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

	_, err := c.do(req)
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

	_, err := c.do(req)
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

	_, err := c.do(req)
	if err != nil {
		return errors.Wrap(err, "RenewDomain: failed to execute a request")
	}

	return err
}

func NewClient() *Client {
	return &Client{c: http.DefaultClient,
		base: setting.GetBaseRdnsURL}
}
