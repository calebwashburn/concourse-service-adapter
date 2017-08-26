package adapter

import (
	"log"

	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type Binder struct {
	StderrLogger *log.Logger
}

//CreateBinding Contract on cf bind-service
func (b Binder) CreateBinding(bindingID string, deploymentTopology bosh.BoshVMs, manifest bosh.BoshManifest, requestParams serviceadapter.RequestParameters) (serviceadapter.Binding, error) {
	prop := manifest.InstanceGroups[0].Properties
	return serviceadapter.Binding{
		Credentials: map[string]interface{}{
			"username": prop["basic_auth_username"],
			"password": prop["basic_auth_password"],
			"host":     prop["external_url"],
		},
	}, nil

}

//DeleteBinding Static credentials no neeed to do anything
func (b Binder) DeleteBinding(bindingID string, deploymentTopology bosh.BoshVMs, manifest bosh.BoshManifest, requestParams serviceadapter.RequestParameters) error {
	return nil
}
