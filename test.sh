service_deployment_json=$(jq -n '
{
    "deployment_name": "service-instance_abcd",
    "releases": [{
        "name": "concourse",
        "version": "0.1.2",
        "jobs": [
            "atc",
            "tsa",
            "baggageclaim",
            "groundcrew",
            "postgresql"
        ]
    },
    {
        "name": "garden-runc",
        "version": "0.1.3",
        "jobs": [
            "garden"
        ]
    },
    {
        "name": "routing",
        "version": "1.4.5",
        "jobs": [
            "route_registrar"
        ]
    }
    ],
    "stemcell": {
        "stemcell_os": "BeOS",
        "stemcell_version": "2"
    }
}
')

plan_json='
{
   "instance_groups": [
      {
         "name": "web",
         "vm_type": "small",
         "networks": [
            "example-network"
         ],
         "azs": [
            "example-az"
         ],
         "instances": 1
      },
      {
         "name": "db",
         "vm_type": "small",
         "networks": [
            "example-network"
         ],
         "azs": [
            "example-az"
         ],
         "instances": 1,
         "persistent_disk_type": "ten"
      },
      {
         "name": "worker",
         "vm_type": "small",
         "networks": [
            "example-network"
         ],
         "azs": [
            "example-az"
         ],
         "instances": 1
      }
   ],
   "properties": {
      "cf_deployment": "test_deployment"
   },
   "update": {
      "canaries": 1,
      "max_in_flight": 2,
      "canary_watch_time": "1000-30000",
      "update_watch_time": "1000-30000",
      "serial": true
  }
}
'

request_params_json='
{
  "parameters": {
    "app_domain": "appsdomain.com"
  }
}
'

./main generate-manifest "$service_deployment_json" "$plan_json" "$request_params_json" "---" "{}"
