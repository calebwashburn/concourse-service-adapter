package adapter

import (
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

// var CurrentPasswordGenerator = randomPasswordGenerator

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

	webInstanceGroup := findInstanceGroup(plan, WebInstanceName)
	webProperties := m.webInstanceProperties(serviceDeployment.DeploymentName, plan.Properties, requestParams.ArbitraryParams(), previousManifest)
	webJobs, err := gatherJobs(serviceDeployment.Releases, AtcJobName)
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
	dbProperties := m.dbInstanceProperties(serviceDeployment.DeploymentName, plan.Properties, requestParams.ArbitraryParams(), previousManifest)
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
	workerJobs, err := gatherJobs(serviceDeployment.Releases, GardenJobName)
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

func gatherJobs(releases serviceadapter.ServiceReleases, jobName string) ([]bosh.Job, error) {
	release, err := findReleaseForJob(jobName, releases)
	if err != nil {
		return nil, err
	}
	return []bosh.Job{{Name: jobName, Release: release.Name}}, nil
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

func (m ManifestGenerator) webInstanceProperties(deploymentName string, planProperties serviceadapter.Properties, arbitraryParams map[string]interface{}, previousManifest *bosh.BoshManifest) map[string]interface{} {
	appDomain := arbitraryParams["app_domain"]
	return map[string]interface{}{
		"external_url":        fmt.Sprintf("https://%s.%s", deploymentName, appDomain),
		"basic_auth_username": "atc",
		"basic_auth_password": "atc",
		"postgresql_database": `&atc_db atc`,
	}
}

func (m ManifestGenerator) dbInstanceProperties(deploymentName string, planProperties serviceadapter.Properties, arbitraryParams map[string]interface{}, previousManifest *bosh.BoshManifest) map[string]interface{} {
	return map[string]interface{}{
		"databases": []map[interface{}]interface{}{
			map[interface{}]interface{}{
				"name":     "*atc_db",
				"role":     "atc",
				"password": "atc",
			},
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
