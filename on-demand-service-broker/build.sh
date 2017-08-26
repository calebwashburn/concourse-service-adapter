
GIT_HUB="$GOPATH/src/github.com"

pushd $GIT_HUB/datianshi/concourse-service-adapter/
  GOOS=linux GOARCH=amd64 go build -o on-demand-service-broker/service_adapter cmd/service-adapter/main.go
popd


pushd $GIT_HUB/pivotal-cf/on-demand-service-broker/
  GOOS=linux GOARCH=amd64 go build -o $GIT_HUB/datianshi/concourse-service-adapter/on-demand-service-broker/broker cmd/on-demand-service-broker/main.go
popd
#cf push testa -c './broker -configFilePath config.yml' -b binary_buildpack

