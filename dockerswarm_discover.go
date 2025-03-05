// Package dockerswarm provides service discovery for Docker Service.
package dockerswarm

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

type Provider struct{}

func (p *Provider) Help() string {
	return `Docker Swarm:

    provider:         "dockerswarm"
    namespace:        Namespace to search for services (defaults to "default").
    service:          Service name to search for.
    network:          Network selector value to filter services (defaults to "{{namespace}}_default").
    host_network:     "true" if service host IP and ports should be used.
`
}

func (p *Provider) Addrs(args map[string]string, l *log.Logger) ([]string, error) {
	if args["provider"] != "dockerswarm" {
		return nil, fmt.Errorf("discover-dockerswarm: invalid provider " + args["provider"])
	}

	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("discover-dockerswarm: %s", err)
	}

	namespace := args["namespace"]
	if namespace == "" {
		namespace = "default"
	}

	service := args["service"]
	if service == "" {
		return nil, fmt.Errorf("discover-dockerswarm: service name is required")
	}
	service = fmt.Sprintf("%s_%s", namespace, service)

	// List all tasks in the Docker Swarm's services.
	tasks, err := cli.TaskList(ctx, types.TaskListOptions{
		Filters: filters.NewArgs(
			filters.Arg("service", service),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("discover-dockerswarm: %s", err)
	}

	return TaskAddrs(tasks, args, l)
}

func TaskAddrs(tasks []swarm.Task, args map[string]string, l *log.Logger) ([]string, error) {
	var addrs []string

	namespace := args["namespace"]
	if namespace == "" {
		namespace = "default"
	}

	service := args["service"]
	if service == "" {
		return nil, fmt.Errorf("discover-dockerswarm: service name is required")
	}
	service = fmt.Sprintf("%s_%s", namespace, service)

	networkSelector := args["network"]
	if networkSelector == "" {
		networkSelector = "default"
	}
	networkSelector = fmt.Sprintf("%s_%s", namespace, networkSelector)

	for _, task := range tasks {
		if task.Status.State != swarm.TaskStateRunning {
			l.Printf("[DEBUG] discover-dockerswarm: ignoring task %q, not ready state", fmt.Sprintf("%s.%d.%s", service, task.Slot, task.ID))
			continue
		}

		if task.NetworksAttachments == nil {
			continue
		}

		for _, network := range task.NetworksAttachments {
			if network.Network.Spec.Name == networkSelector {
				for _, addr := range network.Addresses {
					addr = addr[:len(addr)-3] // Remove the subnet mask.
					if slices.Contains(addrs, addr) {
						continue
					}
					addrs = append(addrs, addr)
				}
			}
		}
	}

	return addrs, nil
}
