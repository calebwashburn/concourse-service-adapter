package adapter

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

const (
	//ConcourseReleaseName name of the job name
	ConcourseReleaseName = "concourse"
	//GardenRuncReleaseName name of the job name
	GardenRuncReleaseName = "garden-runc"
	//WebInstanceName instance name of web
	WebInstanceName = "web"
	//WorkerInstanceName instance name of worker
	WorkerInstanceName = "worker"
	//DatabaseInstanceName instance name of database
	DatabaseInstanceName = "db"
	//AtcJobName atc job name
	AtcJobName = "atc"
	//TsaJobName tsa job name
	TsaJobName = "tsa"
	//PostgresJobName postgre job name
	PostgresJobName = "postgresql"
	//GroundCrewJobName groundcrew job name
	GroundCrewJobName = "groundcrew"
	//BaggageClaimJobName baggageclaim job name
	BaggageClaimJobName = "baggageclaim"
	//GardenJobName garden job name
	GardenJobName = "garden"
)

//CurrentPasswordGenerator Password Generator
var CurrentPasswordGenerator = randomPasswordGenerator

//ManifestGenerator Contract of OMB on manifest generator
type ManifestGenerator struct {
	StderrLogger           *log.Logger
	ConfigPath             string
	RedisInstanceGroupName string
}

func mapNetworksToBoshNetworks(networks []string) []bosh.Network {
	boshNetworks := []bosh.Network{}
	for _, network := range networks {
		boshNetworks = append(boshNetworks, bosh.Network{Name: network})
	}
	return boshNetworks
}

func getProperties(plan serviceadapter.Plan) map[string]interface{} {
	return plan.Properties
}

//GenerateManifest Generate a bosh manifest. Cloud Controller will pass in the arguments
func (m ManifestGenerator) GenerateManifest(
	serviceDeployment serviceadapter.ServiceDeployment,
	plan serviceadapter.Plan,
	requestParams serviceadapter.RequestParameters,
	previousManifest *bosh.BoshManifest,
	previousPlan *serviceadapter.Plan,
) (manifest bosh.BoshManifest, err error) {

	stemcellAlias := "only-stemcell"

	instanceGroups := []bosh.InstanceGroup{}

	releases := []bosh.Release{}
	for _, release := range serviceDeployment.Releases {
		releases = append(releases, bosh.Release{
			Name:    release.Name,
			Version: release.Version,
		})
	}

	webPassword, _ := CurrentPasswordGenerator()
	dbPassword, _ := CurrentPasswordGenerator()

	webInstanceGroup := findInstanceGroup(plan, WebInstanceName)
	webProperties := m.webInstanceProperties(dbPassword, webPassword, serviceDeployment.DeploymentName, plan.Properties, requestParams.ArbitraryParams(), previousManifest)
	webJobs, err := gatherJobs(serviceDeployment.Releases, AtcJobName, TsaJobName)
	if err != nil {
		return
	}
	instanceGroups = append(instanceGroups, bosh.InstanceGroup{
		Name:         WebInstanceName,
		Instances:    webInstanceGroup.Instances,
		Jobs:         webJobs,
		VMType:       webInstanceGroup.VMType,
		VMExtensions: webInstanceGroup.VMExtensions,
		Stemcell:     stemcellAlias,
		Networks:     mapNetworksToBoshNetworks(webInstanceGroup.Networks),
		AZs:          webInstanceGroup.AZs,
		Properties:   webProperties,
	})

	dbInstanceGroup := findInstanceGroup(plan, DatabaseInstanceName)
	dbProperties := m.dbInstanceProperties(dbPassword, serviceDeployment.DeploymentName, plan.Properties, requestParams.ArbitraryParams(), previousManifest)
	dbJobs, err := gatherJobs(serviceDeployment.Releases, PostgresJobName)
	if err != nil {
		return
	}
	instanceGroups = append(instanceGroups, bosh.InstanceGroup{
		Name:               DatabaseInstanceName,
		Instances:          dbInstanceGroup.Instances,
		Jobs:               dbJobs,
		VMType:             dbInstanceGroup.VMType,
		VMExtensions:       dbInstanceGroup.VMExtensions,
		PersistentDiskType: dbInstanceGroup.PersistentDiskType,
		Stemcell:           stemcellAlias,
		Networks:           mapNetworksToBoshNetworks(dbInstanceGroup.Networks),
		AZs:                dbInstanceGroup.AZs,
		Properties:         dbProperties,
	})

	workerInstanceGroup := findInstanceGroup(plan, WorkerInstanceName)
	workerProperties := m.workerInstanceProperties(serviceDeployment.DeploymentName, plan.Properties, requestParams.ArbitraryParams(), previousManifest)
	workerJobs, err := gatherJobs(serviceDeployment.Releases, GroundCrewJobName, BaggageClaimJobName, GardenJobName)
	if err != nil {
		return
	}
	instanceGroups = append(instanceGroups, bosh.InstanceGroup{
		Name:         WorkerInstanceName,
		Instances:    workerInstanceGroup.Instances,
		Jobs:         workerJobs,
		VMType:       workerInstanceGroup.VMType,
		VMExtensions: workerInstanceGroup.VMExtensions,
		Stemcell:     stemcellAlias,
		Networks:     mapNetworksToBoshNetworks(workerInstanceGroup.Networks),
		AZs:          workerInstanceGroup.AZs,
		Properties:   workerProperties,
	})

	return bosh.BoshManifest{
		Name: serviceDeployment.DeploymentName,
		Stemcells: []bosh.Stemcell{
			{
				Alias:   stemcellAlias,
				OS:      serviceDeployment.Stemcell.OS,
				Version: serviceDeployment.Stemcell.Version,
			},
		},
		Releases:       releases,
		InstanceGroups: instanceGroups,
		Update:         generateUpdateBlock(plan.Update, previousManifest),
	}, nil
}

func findInstanceGroup(plan serviceadapter.Plan, instanceGroupName string) *serviceadapter.InstanceGroup {
	for _, instanceGroup := range plan.InstanceGroups {
		if instanceGroup.Name == instanceGroupName {
			return &instanceGroup
		}
	}
	return nil
}

func gatherJobs(releases serviceadapter.ServiceReleases, jobNames ...string) ([]bosh.Job, error) {
	jobs := []bosh.Job{}
	for _, job := range jobNames {
		release, err := findReleaseForJob(job, releases)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, bosh.Job{Name: job, Release: release.Name})
	}
	return jobs, nil
}

func findReleaseForJob(requiredJob string, releases serviceadapter.ServiceReleases) (serviceadapter.ServiceRelease, error) {
	releasesThatProvideRequiredJob := serviceadapter.ServiceReleases{}

	for _, release := range releases {
		for _, providedJob := range release.Jobs {
			if providedJob == requiredJob {
				releasesThatProvideRequiredJob = append(releasesThatProvideRequiredJob, release)
			}
		}
	}

	if len(releasesThatProvideRequiredJob) == 0 {
		return serviceadapter.ServiceRelease{}, fmt.Errorf("no release provided for job %s", requiredJob)
	}

	if len(releasesThatProvideRequiredJob) > 1 {
		releaseNames := []string{}
		for _, release := range releasesThatProvideRequiredJob {
			releaseNames = append(releaseNames, release.Name)
		}

		return serviceadapter.ServiceRelease{}, fmt.Errorf("job %s defined in multiple releases: %s", requiredJob, strings.Join(releaseNames, ", "))
	}

	return releasesThatProvideRequiredJob[0], nil
}

func (m ManifestGenerator) webInstanceProperties(dbPassword string, webPassword string, deploymentName string, planProperties serviceadapter.Properties, arbitraryParams map[string]interface{}, previousManifest *bosh.BoshManifest) map[string]interface{} {
	appDomain := arbitraryParams["app_domain"]
	return map[string]interface{}{
		"external_url":        fmt.Sprintf("https://%s.%s", deploymentName, appDomain),
		"basic_auth_username": "atc",
		"basic_auth_password": webPassword,
		"postgresql_database": generateDatabase(dbPassword),
	}
}

func generateDatabase(dbPassword string) map[interface{}]interface{} {
	return map[interface{}]interface{}{
		"name":     "atc_db",
		"role":     "atc",
		"password": dbPassword,
	}
}

func (m ManifestGenerator) dbInstanceProperties(dbPassword string, deploymentName string, planProperties serviceadapter.Properties, arbitraryParams map[string]interface{}, previousManifest *bosh.BoshManifest) map[string]interface{} {
	return map[string]interface{}{
		"databases": []map[interface{}]interface{}{
			generateDatabase(dbPassword),
		},
	}
}

func (m ManifestGenerator) workerInstanceProperties(deploymentName string, planProperties serviceadapter.Properties, arbitraryParams map[string]interface{}, previousManifest *bosh.BoshManifest) map[string]interface{} {
	return map[string]interface{}{
		"garden": map[interface{}]interface{}{
			"listen_network": "tcp",
			"listen_address": "0.0.0.0:7777",
		},
	}
}

func generateUpdateBlock(update *serviceadapter.Update, previousManifest *bosh.BoshManifest) bosh.Update {
	if update != nil {
		return bosh.Update{
			Canaries:        update.Canaries,
			MaxInFlight:     update.MaxInFlight,
			CanaryWatchTime: update.CanaryWatchTime,
			UpdateWatchTime: update.UpdateWatchTime,
			Serial:          update.Serial,
		}
	}
	updateBlock := bosh.Update{
		Canaries:        4,
		CanaryWatchTime: "30000-240000",
		UpdateWatchTime: "30000-240000",
		MaxInFlight:     4,
	}

	if previousManifest == nil {
		return updateBlock
	}

	updateBlock.Canaries = 1
	updateBlock.MaxInFlight = 1
	return updateBlock

}

func randomPasswordGenerator() (string, error) {
	length := 20
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		log.Printf("Error generating random bytes, %v", err)
		return "", err
	}
	randomStringBytes := make([]byte, base64.StdEncoding.EncodedLen(len(randomBytes)))
	base64.StdEncoding.Encode(randomStringBytes, randomBytes)
	return string(randomStringBytes), nil
}
