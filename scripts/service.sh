cf push testa -c './broker -configFilePath config.yml' -b binary_buildpack
cf create-service-broker concourser admin admin https://testa.cfapps.haas-60.pez.pivotal.io
cf enable-service-access concourse-on-demand-service
cf create-service concourse-on-demand-service small my-concourse -c '{"app_domain":"testa.cfapps.haas-60.pez.pivotal.io"}
