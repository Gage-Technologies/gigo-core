package main

import (
	"fmt"
	"gigo-core/gigo/config"
	"strings"

	"github.com/gage-technologies/gigo-lib/workspace_config"
	"gopkg.in/yaml.v3"
)

const d = `
version: 0.1
resources:
  # this project can be run on free but it is suggested to be run on pro
  # free users will have reduced performance since they will not be able
  # to use larger resources
  cpu: 4
  mem: 6
  disk: 25
base_container: gigodev/gimg:python-ubuntu
working_directory: /home/gigo/codebase
environment: {}
containers:
  version: '3.7'
  services:
    mongodb_container:
      image: 'mongo:latest'
      environment:
        MONGO_INITDB_ROOT_USERNAME: root
        MONGO_INITDB_ROOT_PASSWORD: rootpassword
      ports:
        - 27017:27017
      volumes:
        - /home/gigo/.gigo/containers/data/mongodb:/data/db
vscode:
  enabled: true
  extensions:
  - ms-python.python
  - ms-vscode.cpptools
port_forward: []
exec:
  - name: "Install Python Dependencies"
    init: true
    command: "pip install -r requirements.txt"
  - name: "Install Python Development Dependencies"
    init: true
    command: "pip install -r dev-requirements.txt"
`

func handleRegistryCaches(containerName string, caches []config.RegistryCacheConfig) string {
	// create a variable to hold the docker.io cache if it exists
	var dockerCache config.RegistryCacheConfig

	// iterate over the registry caches
	for _, cache := range caches {
		// if the container name contains the registry host
		if strings.HasPrefix(containerName, cache.Source) {
			// replace the registry host with the cache host
			return strings.Replace(containerName, cache.Source, cache.Cache, 1)
		}

		// save the docker cache if it exists in case the container has no host prefix
		if cache.Source == "docker.io" {
			// set the docker cache
			dockerCache = cache
		}
	}

	// if the container name has no host prefix and the docker cache exists
	// then we assume the container is from docker.io and prepend the cache
	if dockerCache.Source == "docker.io" && strings.Count(containerName, "/") <= 1 {
		return fmt.Sprintf("%s/%s", dockerCache.Cache, containerName)
	}

	// return the container name if no cache was found
	return containerName
}

func main() {
	var cfg workspace_config.GigoWorkspaceConfig
	err := yaml.Unmarshal([]byte(d), &cfg)
	if err != nil {
		panic(err)
	}

	if services, ok := cfg.Containers["services"]; ok {
		// only proceed if we can assert services as a map[string]interface{}
		if servicesMap, ok := services.(map[string]interface{}); ok {
			// iterate over the container services
			for _, service := range servicesMap {
				// try to assert the service as a map[string]interface{}
				if serviceMap, ok := service.(map[string]interface{}); ok {
					// if the service has an image key
					if image, ok := serviceMap["image"]; ok {
						// try to assert the image as a string
						if imageName, ok := image.(string); ok {
							// handle the registry caches
							serviceMap["image"] = handleRegistryCaches(imageName, []config.RegistryCacheConfig{
								{
									Source: "docker.io",
									Cache:  "harbor.gigo.dev/docker.io",
								},
							})
						}
					}
				}
			}
		}
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(b))
}
