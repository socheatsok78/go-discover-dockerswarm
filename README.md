# About
A `dockerswarm` provider for https://github.com/hashicorp/go-discover

```
Docker Swarm:

    provider:         "dockerswarm"
    host:             "tcp://host:port" or "unix:///path/to/socket" (defaults to "unix:///var/run/docker.sock").

    type:             "node"
    role:             "manager" or "worker" (defaults to "").

    type:             "service"
    namespace:        Namespace to search for services (defaults to "default").
    name:             Service name to search for.
    network:          Network selector value to filter services (defaults to "{{namespace}}_default").
    host_network:     "true" if service host IP and ports should be used.
```
