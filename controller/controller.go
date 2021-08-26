package controller

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"strings"
)

type OldContainerConfig struct {
	ContainerName string
	ContainerConfig *container.Config
	ContainerHostConfig *container.HostConfig
}

type DockerController struct {
	cli *client.Client
	ctx context.Context
}

func New(cli *client.Client, ctx context.Context) *DockerController {
	return &DockerController{cli: cli, ctx: ctx}
}

func (dc *DockerController) FindContainerIDByName(containerName string) string {
	containers, err := dc.cli.ContainerList(dc.ctx, types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		if container.Names[0][1:] == containerName {
			return container.ID
		}
	}

	return ""
}

func (dc *DockerController) copyContainerConfig(containerId string) (OldContainerConfig, error) {
	containerJson, err := dc.cli.ContainerInspect(dc.ctx, containerId)

	if err != nil {
		return OldContainerConfig{}, nil
	}

	return OldContainerConfig{
		ContainerConfig:     containerJson.Config,
		ContainerHostConfig: containerJson.HostConfig,
		ContainerName: containerJson.ContainerJSONBase.Name,
	}, nil
}

func (config *OldContainerConfig) setImageTag(imageTag string) {
	imageSplit := strings.Split(config.ContainerConfig.Image, ":")
	baseImage := imageSplit[0]

	config.ContainerConfig.Image = 	fmt.Sprintf("%s:%s", baseImage, imageTag)
}

func (dc *DockerController) UpdateContainer(containerId, imageTag string) error {
	fmt.Printf("copying config from %s\n", containerId)
	configCopy, err := dc.copyContainerConfig(containerId)
	if err != nil {
		return fmt.Errorf("couldn't copy container confi: %w", err)
	}

	fmt.Printf("renaming %s (%s) to %s-old\n", configCopy.ContainerName, containerId, configCopy.ContainerName)
	if err = dc.renameContainer(containerId, configCopy.ContainerName+"-old"); err != nil {
		return fmt.Errorf("couldn't rename container: %w", err)
	}

	configCopy.setImageTag(imageTag)

	fmt.Println("creating new container...")
	newContainerId, err := dc.createContainer(configCopy)
	if err != nil {
		return err
	}

	fmt.Println("updated container id:", newContainerId)

	fmt.Printf("stopping %s-old (%s)\n", configCopy.ContainerName, containerId)
	if err = dc.stopContainer(containerId); err != nil {
		return fmt.Errorf("coulnd't stop container %s: %w", configCopy.ContainerName, err)
	}

	fmt.Printf("starting new container (%s)\n", newContainerId)
	if err = dc.startContainer(newContainerId); err != nil {
		return err
	}

	isContainerRunning := dc.isContainerRunning(newContainerId)

	if !isContainerRunning {
		fmt.Println("new container is not running, trying to restore old container...")
		if err := dc.restoreContainer(containerId, newContainerId, configCopy.ContainerName); err != nil {
			return fmt.Errorf("couldn't restore old container: %w", err)
		}

		fmt.Println("successfully restored container")
		return fmt.Errorf("the new container didn't run, restored old container successfully")
	}

	fmt.Printf("removing container %s-old (%s)\n", configCopy.ContainerName, containerId)
	err = dc.removeContainer(containerId)
	if err != nil {
		return fmt.Errorf("couldn't remove container %s-old: %w", containerId, err)
	}

	return nil
}

func (dc *DockerController) restoreContainer(oldContainerId, newContainerId, originalName string) error {
	fmt.Printf("removing container %s\n", newContainerId)
	if err := dc.removeContainer(newContainerId); err != nil {
		return err
	}

	fmt.Printf("renaming %s to %s\n", oldContainerId, originalName)
	if err := dc.renameContainer(oldContainerId, originalName); err != nil {
		return err
	}

	fmt.Printf("starting container %s\n", oldContainerId)
	if err := dc.startContainer(oldContainerId); err != nil {
		return err
	}

	return nil
}

func (dc *DockerController) removeContainer(containerId string) error {
	return dc.cli.ContainerRemove(dc.ctx, containerId, types.ContainerRemoveOptions{})
}

func (dc *DockerController) stopContainer(containerId string) error {
	return dc.cli.ContainerStop(dc.ctx, containerId, nil)
}

func (dc *DockerController) renameContainer(containerId, newName string) error {
	return dc.cli.ContainerRename(dc.ctx, containerId, newName)
}

func (dc *DockerController) createContainer(config OldContainerConfig) (string, error) {
	resp, err := dc.cli.ContainerCreate(dc.ctx, config.ContainerConfig, config.ContainerHostConfig, nil, nil, config.ContainerName)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (dc *DockerController) startContainer(containerId string) error {
	if err := dc.cli.ContainerStart(dc.ctx, containerId, types.ContainerStartOptions{}); err != nil {
		return err
	}

	return nil
}

func (dc *DockerController) isContainerRunning(containerId string) bool {
	containers, err := dc.cli.ContainerList(dc.ctx, types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		if container.ID == containerId {
			return true
		}
	}

	return false
}