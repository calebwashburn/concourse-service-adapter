package adapter_test

import (
	"io"
	"log"
	"strings"

	"github.com/datianshi/concourse-service-adapter/adapter"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Concourse Service Adapter", func() {

	const ProvidedRedisServerInstanceGroupName = "redis-server"

	var (
		defaultServiceReleases   serviceadapter.ServiceReleases
		defaultRequestParameters map[string]interface{}
		manifestGenerator        adapter.ManifestGenerator
		binder                   adapter.Binder
		concoursePlan            serviceadapter.Plan
		stderr                   *gbytes.Buffer
		stderrLogger             *log.Logger
	)

	BeforeEach(func() {
		concoursePlan = serviceadapter.Plan{
			Properties: map[string]interface{}{},
			InstanceGroups: []serviceadapter.InstanceGroup{
				{
					Name:      "web",
					VMType:    "medium",
					Networks:  []string{"default_network"},
					Instances: 42,
					AZs:       []string{"az1"},
				},
				{
					Name:      "db",
					VMType:    "medium",
					Networks:  []string{"default_network"},
					Instances: 42,
					AZs:       []string{"az1"},
				},
				{
					Name:      "worker",
					VMType:    "medium",
					Networks:  []string{"default_network"},
					Instances: 42,
					AZs:       []string{"az1"},
				},
			},
		}

		defaultRequestParameters = map[string]interface{}{
			"parameters": map[string]interface{}{
				"app_domain": "systemdomain.com",
			},
		}

		defaultServiceReleases = serviceadapter.ServiceReleases{
			{
				Name:    adapter.ConcourseReleaseName,
				Version: "4",
				Jobs: []string{
					adapter.AtcJobName,
					adapter.TsaJobName,
					adapter.PostgresJobName,
					adapter.BaggageClaimJobName,
					adapter.GroundCrewJobName,
				},
			},
			{
				Name:    adapter.GardenRuncReleaseName,
				Version: "3",
				Jobs: []string{
					adapter.GardenJobName,
				},
			},
		}

		stderr = gbytes.NewBuffer()
		stderrLogger = log.New(io.MultiWriter(stderr, GinkgoWriter), "", log.LstdFlags)

		manifestGenerator = createManifestGenerator("concourse-service-adapter.conf", stderrLogger)

		binder = adapter.Binder{StderrLogger: stderrLogger}
	})

	Describe("Generating manifests", func() {
		It("Setup the correct releases", func() {
			oldManifest := createDefaultOldManifest()
			generated, generateErr := generateManifest(
				manifestGenerator,
				defaultServiceReleases,
				concoursePlan,
				defaultRequestParameters,
				&oldManifest,
				nil,
			)

			expectReleases := []bosh.Release{bosh.Release{
				Name:    "concourse",
				Version: "4",
			},
				bosh.Release{
					Name:    "garden-runc",
					Version: "3",
				},
			}

			Expect(generateErr).NotTo(HaveOccurred())
			Expect(generated.Name).To(Equal("some-instance-id"))
			Expect(generated.Releases).To(Equal(expectReleases))
		})

		It("Setup the correct stemcell", func() {
			oldManifest := createDefaultOldManifest()
			generated, generateErr := generateManifest(
				manifestGenerator,
				defaultServiceReleases,
				concoursePlan,
				defaultRequestParameters,
				&oldManifest,
				nil,
			)

			expectStemcells := []bosh.Stemcell{
				bosh.Stemcell{
					Alias:   "only-stemcell",
					OS:      "some-stemcell-os",
					Version: "1234",
				},
			}

			Expect(generateErr).NotTo(HaveOccurred())
			Expect(generated.Stemcells).To(Equal(expectStemcells))
		})

		It("sets the concourse web tier instance group", func() {
			oldManifest := createDefaultOldManifest()
			generated, generateErr := generateManifest(
				manifestGenerator,
				defaultServiceReleases,
				concoursePlan,
				defaultRequestParameters,
				&oldManifest,
				nil,
			)

			Expect(generateErr).NotTo(HaveOccurred())
			Expect(generated.InstanceGroups[0].Name).To(Equal(adapter.WebInstanceName))
			Expect(generated.InstanceGroups[0].Properties["external_url"]).To(Equal("https://some-instance-id.systemdomain.com"))
			Expect(generated.InstanceGroups[0].Jobs[0].Name).To(Equal(adapter.AtcJobName))
			Expect(generated.InstanceGroups[0].Jobs[1].Name).To(Equal(adapter.TsaJobName))
		})

		It("sets the concourse db tier instance group", func() {
			oldManifest := createDefaultOldManifest()
			generated, generateErr := generateManifest(
				manifestGenerator,
				defaultServiceReleases,
				concoursePlan,
				defaultRequestParameters,
				&oldManifest,
				nil,
			)

			Expect(generateErr).NotTo(HaveOccurred())
			Expect(generated.InstanceGroups[1].Name).To(Equal(adapter.DatabaseInstanceName))
			databaseName := generated.InstanceGroups[1].Properties["databases"].([]map[interface{}]interface{})[0]["name"]
			Expect(databaseName).To(Equal("atc_db"))
			Expect(generated.InstanceGroups[1].Jobs[0].Name).To(Equal(adapter.PostgresJobName))
		})

		It("sets the concourse worker tier instance group", func() {
			oldManifest := createDefaultOldManifest()
			generated, generateErr := generateManifest(
				manifestGenerator,
				defaultServiceReleases,
				concoursePlan,
				defaultRequestParameters,
				&oldManifest,
				nil,
			)

			Expect(generateErr).NotTo(HaveOccurred())
			Expect(generated.InstanceGroups[2].Name).To(Equal(adapter.WorkerInstanceName))

			listener_network := generated.InstanceGroups[2].Properties["garden"].(map[interface{}]interface{})["listen_network"]
			listen_address := generated.InstanceGroups[2].Properties["garden"].(map[interface{}]interface{})["listen_address"]

			Expect(listener_network).To(Equal("tcp"))
			Expect(listen_address).To(Equal("0.0.0.0:7777"))

			Expect(generated.InstanceGroups[2].Jobs[0].Name).To(Equal(adapter.GroundCrewJobName))
			Expect(generated.InstanceGroups[2].Jobs[1].Name).To(Equal(adapter.BaggageClaimJobName))
			Expect(generated.InstanceGroups[2].Jobs[2].Name).To(Equal(adapter.GardenJobName))

		})

	})

	Describe("binding", func() {
		var (
			actualBinding    serviceadapter.Binding
			actualBindingErr error
			boshVMs          bosh.BoshVMs
			currentManifest  bosh.BoshManifest
		)
		BeforeEach(func() {

			boshVMs = bosh.BoshVMs{}
			currentManifest = bosh.BoshManifest{
				InstanceGroups: []bosh.InstanceGroup{
					{
						Properties: map[string]interface{}{
							"basic_auth_username": "atc",
							"basic_auth_password": "password",
							"external_url":        "host",
						},
					},
				},
			}

		})

		JustBeforeEach(func() {
			actualBinding, actualBindingErr = binder.CreateBinding("not-relevant", boshVMs, currentManifest, nil)
		})

		Context("has a password in the manifest", func() {
			It("has no error", func() {
				Expect(actualBindingErr).NotTo(HaveOccurred())
			})

			It("returns the username from the manifest", func() {
				Expect(actualBinding.Credentials["username"]).To(Equal("atc"))
			})

			It("returns the password from the manifest", func() {
				Expect(actualBinding.Credentials["password"]).To(Equal("password"))
			})

			It("returns the host from the vms", func() {
				Expect(actualBinding.Credentials["host"]).To(Equal("host"))
			})
		})
	})

})

func createManifestGenerator(filename string, logger *log.Logger) adapter.ManifestGenerator {
	return adapter.ManifestGenerator{
		StderrLogger: logger,
		ConfigPath:   getFixturePath(filename),
	}
}

func createDefaultEmptyManifest() bosh.BoshManifest {
	return bosh.BoshManifest{}
}

func createDefaultOldManifest() bosh.BoshManifest {
	return bosh.BoshManifest{
		Releases: []bosh.Release{
			{Name: "some-release-name", Version: "4"},
		},
		InstanceGroups: []bosh.InstanceGroup{},
	}
}

func getFixturePath(filename string) string {
	cwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	return filepath.Join(cwd, "fixtures", filename)
}

func planWithPropertyRemoved(plan serviceadapter.Plan, property string) serviceadapter.Plan {
	propertySlice := strings.Split(property, ".")
	if len(propertySlice) == 1 {
		delete(plan.Properties, property)
	} else {
		delete(plan.Properties[propertySlice[0]].(map[string]interface{}), propertySlice[1])
	}
	return plan
}

func generateManifest(
	manifestGenerator adapter.ManifestGenerator,
	serviceReleases serviceadapter.ServiceReleases,
	plan serviceadapter.Plan,
	requestParams map[string]interface{},
	oldManifest *bosh.BoshManifest,
	oldPlan *serviceadapter.Plan,
) (bosh.BoshManifest, error) {

	return manifestGenerator.GenerateManifest(serviceadapter.ServiceDeployment{
		DeploymentName: "some-instance-id",
		Stemcell: serviceadapter.Stemcell{
			OS:      "some-stemcell-os",
			Version: "1234",
		},
		Releases: serviceReleases,
	}, plan, requestParams, oldManifest, oldPlan)
}
