package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// Manager wraps the Docker client for Swarm service management.
type Manager struct {
	cli *client.Client
}

// NewManager creates a Docker manager connected to the local Docker daemon.
func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Manager{cli: cli}, nil
}

// Close releases the Docker client connection.
func (m *Manager) Close() error {
	return m.cli.Close()
}

// ListServices returns all IIoT MQTT reader services in the swarm.
func (m *Manager) ListServices(ctx context.Context) ([]swarm.Service, error) {
	services, err := m.cli.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "name",
			Value: "iiot_mqtt-reader",
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	return services, nil
}

// GetService returns a specific MQTT reader service by name.
func (m *Manager) GetService(ctx context.Context, name string) (*swarm.Service, error) {
	// Docker stack prefixes service names with the stack name
	serviceName := fmt.Sprintf("iiot_mqtt-reader-%s", name)
	service, _, err := m.cli.ServiceInspectWithRaw(ctx, serviceName, types.ServiceInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get service %s: %w", serviceName, err)
	}
	return &service, nil
}

// RemoveService removes an MQTT reader service.
func (m *Manager) RemoveService(ctx context.Context, name string) error {
	serviceName := fmt.Sprintf("iiot_mqtt-reader-%s", name)
	return m.cli.ServiceRemove(ctx, serviceName)
}

// RestartService force-restarts an MQTT reader service by updating it.
func (m *Manager) RestartService(ctx context.Context, name string) error {
	serviceName := fmt.Sprintf("iiot_mqtt-reader-%s", name)
	service, _, err := m.cli.ServiceInspectWithRaw(ctx, serviceName, types.ServiceInspectOptions{})
	if err != nil {
		return fmt.Errorf("inspect service %s: %w", serviceName, err)
	}

	// Force update to trigger restart
	service.Spec.TaskTemplate.ForceUpdate++
	_, err = m.cli.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		return fmt.Errorf("update service %s: %w", serviceName, err)
	}
	return nil
}

// ServiceInfo holds summary information about a running service.
type ServiceInfo struct {
	Name      string `json:"name"`
	Image     string `json:"image"`
	Replicas  uint64 `json:"replicas"`
	Ready     uint64 `json:"ready"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ListReaderServices returns a user-friendly list of MQTT reader services.
func (m *Manager) ListReaderServices(ctx context.Context) ([]ServiceInfo, error) {
	services, err := m.ListServices(ctx)
	if err != nil {
		return nil, err
	}

	var result []ServiceInfo
	for _, svc := range services {
		name := svc.Spec.Name
		// Strip "iiot_mqtt-reader-" prefix for display
		shortName := strings.TrimPrefix(name, "iiot_mqtt-reader-")
		if shortName == name {
			shortName = strings.TrimPrefix(name, "iiot_")
		}

		replicas := uint64(0)
		ready := uint64(0)
		if svc.Spec.Mode.Replicated != nil {
			replicas = svc.Spec.Mode.Replicated.Replicas
		}
		// Service status from service info
		if svc.ServiceStatus != nil {
			ready = svc.ServiceStatus.DesiredTasks
		}

		image := ""
		if svc.Spec.TaskTemplate.ContainerSpec != nil {
			image = svc.Spec.TaskTemplate.ContainerSpec.Image
		}

		status := "running"
		if ready == 0 {
			status = "pending"
		}

		result = append(result, ServiceInfo{
			Name:      shortName,
			Image:     image,
			Replicas:  replicas,
			Ready:     ready,
			Status:    status,
			CreatedAt: svc.CreatedAt.String(),
		})
	}

	return result, nil
}

// Info returns basic Docker system info.
func (m *Manager) Info(ctx context.Context) (map[string]any, error) {
	info, err := m.cli.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("docker info: %w", err)
	}

	return map[string]any{
		"swarm_active":  info.Swarm.LocalNodeState == "active",
		"node_id":       info.Swarm.NodeID,
		"server_version": info.ServerVersion,
		"containers":    info.Containers,
		"arch":          info.Architecture,
		"os":            info.OSType,
	}, nil
}
