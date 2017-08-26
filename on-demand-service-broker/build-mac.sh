GIT_HUB="$GOPATH/src/github.com"
pushd $GIT_HUB/datianshi/concourse-service-adapter/
  go build -o on-demand-service-broker/service_adapter_mac cmd/service-adapter/main.go
popd
pushd $GIT_HUB/pivotal-cf/on-demand-service-broker/
  go build -o $GIT_HUB/datianshi/concourse-service-adapter/on-demand-service-broker/broker-mac cmd/on-demand-service-broker/main.go
popd
./broker-mac -configFilePath config.yml 
