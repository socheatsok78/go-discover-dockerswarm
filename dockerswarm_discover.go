// Package dockerswarm provides service discovery for Docker Service.
package dockerswarm

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

type Provider struct{}

func (p *Provider) Help() string {
	return `Docker Swarm:

    provider:         "dockerswarm"
    host:             "tcp://host:port" or "unix:///path/to/socket" (defaults to "unix:///var/run/docker.sock").

    type:             "node"
    role:             "manager" or "worker" (defaults to "").

    type:             "service"
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

	host := args["host"]
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	hostURL, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("discover-dockerswarm: invalid host URL %q: %s", host, err)
	}

	opts := []client.Opt{
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	}

	// There are other protocols than HTTP supported by the Docker daemon, like
	// unix, which are not supported by the HTTP client. Passing HTTP client
	// options to the Docker client makes those non-HTTP requests fail.
	if hostURL.Scheme == "http" || hostURL.Scheme == "https" {
		opts = append(opts,
			client.WithHTTPClient(&http.Client{
				Timeout: time.Duration(30) * time.Second,
			}),
			client.WithScheme(hostURL.Scheme),
		)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("discover-dockerswarm: %s", err)
	}
	defer cli.Close()

	discoverType := args["type"]

	switch discoverType {
	case "node":
		return NodeAddrs(cli, args, l)
	case "service":
		return ServiceAddrs(cli, args, l)
	default:
		return nil, fmt.Errorf("discover-dockerswarm: invalid type %q", discoverType)
	}
}

func NodeAddrs(cli *client.Client, args map[string]string, l *log.Logger) ([]string, error) {
	ctx := context.Background()

	filters := filters.NewArgs()

	role := args["role"]
	if role != "" {
		filters.Add("role", role)
	}

	nodes, err := cli.NodeList(ctx, types.NodeListOptions{
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("discover-dockerswarm: %s", err)
	}

	var addrs []string

	for _, node := range nodes {
		if node.Status.State != swarm.NodeStateReady {
			l.Printf("[DEBUG] discover-dockerswarm: ignoring node %q, not ready state", node.Description.Hostname)
			continue
		}
		if node.Spec.Availability != swarm.NodeAvailabilityActive {
			l.Printf("[DEBUG] discover-dockerswarm: ignoring node %q, not active availability", node.Description.Hostname)
			continue
		}
		addrs = append(addrs, node.Status.Addr)
	}

	return addrs, nil
}

func ServiceAddrs(cli *client.Client, args map[string]string, l *log.Logger) ([]string, error) {
	ctx := context.Background()

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
		l.Printf("[DEBUG] discover-dockerswarm: using default namespace")
		namespace = "default"
	}

	service := args["service"]
	if service == "" {
		return nil, fmt.Errorf("discover-dockerswarm: service name is required")
	}
	service = fmt.Sprintf("%s_%s", namespace, service)

	networkSelector := args["network"]
	if networkSelector == "" {
		l.Printf("[DEBUG] discover-dockerswarm: using default network")
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
