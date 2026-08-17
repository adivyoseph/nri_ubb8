//AMD_Strix_halo_nri2026/08/16 07:58:52 SYNC Container name vllm,
// podId fc166dab31dc7a22f94ec6046d92aa0117ec441f878d1b7f0695fdfca78aaa9d,
// cpus shares:{value:6144} quota:{value:1000000} period:{value:100000}
//YNC Container  devixes [
// path:"/dev/kfd" type:"c" major:234 file_mode:{value:432} uid:{} gid:{value:991}
//  path:"/dev/dri/card1" type:"c" major:226 minor:1 file_mode:{value:432} uid:{} gid:{value:44}
// path:"/dev/dri/renderD128" type:"c" major:226 minor:128 file_mode:{value:432} uid:{} gid:{value:991}]
// log.Printf("SYNC Container  devixes %s\n",
//			container.GetLinux().GetDevices())

package plugin

import (
	"context"
	"fmt"
	"log"

	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"

	allocate "github.com/adivyoseph/nri_ubb8/pkg/allocate"
)

const (
	pluginName = "amd-uub8-plugin"
	pluginIdx  = "01" // Plugin execution order (00-99)
)

type plugin struct {
	stub stub.Stub
}

// Configure is called when the plugin is configured
func (p *plugin) Configure(ctx context.Context, config, runtime, version string) (stub.EventMask, error) {
	log.Printf("Plugin configuration event  config = %s, runtime = %s, version = %s\n", config, runtime, version)

	// Subscribe to all events
	// unhandled events UpdatePodSandbox,PostUpdatePodSandbox,ValidateContainerAdjustment
	return api.MustParseEventMask(
		//"all",
		//*
		"RunPodSandbox",
		"StopPodSandbox",
		"RemovePodSandbox",
		"CreateContainer",
		"PostCreateContainer",
		"StartContainer",
		"PostStartContainer",
		"UpdateContainer",
		"PostUpdateContainer",
		"StopContainer",
		"RemoveContainer",
		//"StateChange",
		//*/
	), nil
}

// getContainerCpusetCpus reads the effective cpuset assigned to a container's cgroup
func getContainerCpusetCpus(pod *api.PodSandbox, container *api.Container) (string, error) {
	cgroupsPath := container.GetLinux().GetCgroupsPath()
	if cgroupsPath == "" {
		return "", fmt.Errorf("container %s has no cgroups path", container.GetId())
	}

	var fullPath string
	if strings.HasPrefix(cgroupsPath, "/") {
		fullPath = filepath.Join("/sys/fs/cgroup", cgroupsPath)
	} else {
		fullPath = filepath.Join("/sys/fs/cgroup", pod.GetLinux().GetCgroupParent(), cgroupsPath)
	}

	data, err := os.ReadFile(filepath.Join(fullPath, "cpuset.cpus.effective"))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// Synchronize is called to synchronize plugin state with runtime
func (p *plugin) Synchronize(ctx context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	log.Printf("Plugin synchronization event pods_count = %d, containers_count = %d\n", len(pods), len(containers))

	var updates []*api.ContainerUpdate

	//just care about containers
	for _, container := range containers {
		log.Printf("SYNC Container name %s, podId %s,  cpus %s\n",
			container.GetName(),
			container.PodSandboxId,
			container.GetLinux().GetResources().GetCpu())
		//log.Printf("SYNC Container  devices %s\n",
		//	container.GetLinux().GetDevices())
		// update resources consumed in allocate

		cpuSet := ""
		numa := "0"

		//find pod
		for _, pod := range pods {
			if container.PodSandboxId == pod.Id {
				annotations := pod.GetAnnotations()
				if val, ok := annotations["amd.llm"]; ok && val == "yes" {
					devices := container.GetLinux().GetDevices()
					for _, d := range devices {
						path := d.GetPath()
						_, cardNum, cardFound := strings.Cut(path, "card")
						if cardFound {
							log.Printf("SYNC Container %s %s\n", path, cardNum)

						}
						if val, ok := annotations["amd.smt"]; ok && val == "off" {
							cpuSet, numa = allocate.ReserveCard(pod, container, path, 0)
						} else {
							cpuSet, numa = allocate.ReserveCard(pod, container, path, 1)
						}
						update := &api.ContainerUpdate{}
						update.SetContainerId(container.GetId())
						update.SetLinuxCPUSetCPUs(cpuSet)
						update.SetLinuxCPUSetMems(numa) // REQUIRED: Set memory nodes for cpuset, always numa 0
						updates = append(updates, update)

					}
				} else {
					//regular containers
					//TODO bin pack
				}
				break
			}
		}
	}

	return updates, nil
}

///+++++++++++++++++++++ POD event
/// ignore all
/// TODO remove

// RunPodSandbox handles pod sandbox creation
func (p *plugin) RunPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	log.Printf("POD creation sandbox event: pod_name %s, pod_namespace %s, pod_uid %s\n", pod.GetName(), pod.GetNamespace(), pod.GetUid())

	// Enable cpuset controller in the pod's cgroup
	// Pod cgroup path is typically: /sys/fs/cgroup/kubepods.slice/kubepods-<qos>.slice/kubepods-<qos>-pod<uid>.slice
	log.Printf("POD  cgroupParent %s\n", pod.Linux.CgroupParent)

	log.Printf("POD  cgroupsPath %s\n", pod.Linux.CgroupsPath)

	//pod.Linux.PodResources.Cpu.Cpus
	//file, err := os.ReadFile("config.yaml")
	//if err != nil {
	//	log.Fatal(err)
	//}

	if pod.Linux != nil && pod.Linux.CgroupParent != "" {

		podCgroupPath := "/sys/fs/cgroup/" + pod.Linux.CgroupParent
		subtreeControlPath := podCgroupPath + "/cgroup.subtree_control"

		log.Printf("POD    Enabling cpuset in pod cgroup: %s\n", podCgroupPath)

		// Write "+cpuset" to enable the cpuset controller for child cgroups
		err := os.WriteFile(subtreeControlPath, []byte("+cpuset"), 0644)
		if err != nil {
			log.Printf("\tWARNING: Failed to enable cpuset controller in pod cgroup: %v\n", err)
		} else {
			log.Printf("\tSUCCESS: Enabled cpuset controller in pod cgroup\n")
		}
	}

	return nil
}

// StopPodSandbox handles pod sandbox stop
func (p *plugin) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	log.Printf("POD stop sandbox event: pod_name %s, pod_namespace %s, pod_uid %s \n", pod.GetName(), pod.GetNamespace(), pod.GetUid())

	//may come before container stop
	/*if allocate.IsPodReserved(pod.GetNamespace()) {
		allocate.RemovePodFromReserved(pod)
	} else {
		allocate.RemovePod(pod)
	}
	*/
	return nil
}

// RemovePodSandbox handles pod sandbox removal
func (p *plugin) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	log.Printf("POD remove sandbox event: pod_name %s, pod_namespace %s, pod_uid %s \n", pod.GetName(), pod.GetNamespace(), pod.GetUid())

	return nil
}

///++++++++++++++++++++++++++ Container events

// CreateContainer handles container creation
func (p *plugin) CreateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	log.Printf("   Container creation event pod_name %s, pod_uid %s, container_id %s, container_name %s\n",
		pod.GetName(),
		//pod.GetNamespace(),
		pod.GetUid(),
		container.GetId(),
		container.GetName())
	log.Printf("   Container  CgroupsPath %s, cpusShares %d \n", container.Linux.GetCgroupsPath(), container.Linux.Resources.Cpu.Shares.GetValue())

	var updates []*api.ContainerUpdate
	cpuSet := ""
	numa := "0"

	//look for annotations
	annotations := pod.GetAnnotations()
	log.Printf("Pod annotations %v\n", annotations)

	if val, ok := annotations["amd.llm"]; ok && val == "yes" {
		log.Printf("Pod is an llm\n")
		devices := container.GetLinux().GetDevices()
		for _, d := range devices {
			path := d.GetPath()
			_, cardNum, cardFound := strings.Cut(path, "card")
			if cardFound {
				log.Printf("SYNC Container %s %s\n", path, cardNum)
			}
			if val, ok := annotations["amd.smt"]; ok && val == "off" {
				cpuSet, numa = allocate.ReserveCard(pod, container, path, 0)
			} else {
				cpuSet, numa = allocate.ReserveCard(pod, container, path, 1)
			}
			update := &api.ContainerUpdate{}
			update.SetContainerId(container.GetId())
			update.SetLinuxCPUSetCPUs(cpuSet)
			update.SetLinuxCPUSetMems(numa) // REQUIRED: Set memory nodes for cpuset, always numa 0
			updates = append(updates, update)
		}

	} else {

		cpuSet, numa := allocate.AddContainerSharedPool(pod, container)
		if len(numa) > 0 {
			update := &api.ContainerUpdate{}
			update.SetContainerId(container.GetId())
			update.SetLinuxCPUSetCPUs(cpuSet)
			update.SetLinuxCPUSetMems(numa) // REQUIRED: Set memory nodes for cpuset, always numa 0
			updates = append(updates, update)
			log.Printf("   ContainerB update.\n")
		} else {
			log.Printf("   ContainerB GetAllocation failed\n")
		}
	}

	//allocate.DumpReserved()
	// Return nil to indicate no adjustments needed
	// You can modify container settings here by returning a ContainerAdjustment
	return nil, updates, nil
}

// UpdateContainer handles container updates
//
//	UpdateContainer(context.Context, *api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error)
func (p *plugin) UpdateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container, linuxres *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	log.Printf("\tContainer update event pod_name %s, pod_uid %s, container_id %s, container_name %s\n",
		pod.GetName(),
		//pod.GetNamespace(),
		pod.GetUid(),
		container.GetId(),
		container.GetName())

	// Return nil to indicate no updates needed
	return nil, nil
}

// StopContainer handles container stop events
func (p *plugin) StopContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	log.Printf("\tContainer stop event pod_name %s, pod_uid %s, container_id %s, container_name %s\n",
		pod.GetName(),
		//pod.GetNamespace(),
		pod.GetUid(),
		container.GetId(),
		container.GetName())

	return nil, nil
}

// RemoveContainer handles container removal
func (p *plugin) RemoveContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	log.Printf("\tContainer removal event pod_name %s, pod_uid %s, container_id %s, container_name %s\n",
		pod.GetName(),
		//pod.GetNamespace(),
		pod.GetUid(),
		container.GetId(),
		container.GetName())

	allocate.RemoveContainer(pod, container)

	return nil
}

// PostCreateContainer is called after container creation
func (p *plugin) PostCreateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	log.Printf("\tPost container creation event pod_name %s, pod_uid %s, container_id %s, container_name %s\n",
		pod.GetName(),
		//pod.GetNamespace(),
		pod.GetUid(),
		container.GetId(),
		container.GetName())

	return nil
}

// PostStartContainer is called after container starts
func (p *plugin) PostStartContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	log.Printf("\tPost container start event pod_name %s, pod_uid %s, container_id %s, container_name %s\n",
		pod.GetName(),
		//pod.GetNamespace(),
		pod.GetUid(),
		container.GetId(),
		container.GetName())

	return nil
}

// PostUpdateContainer is called after container update
func (p *plugin) PostUpdateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	log.Printf("\tPost container update event pod_name %s, pod_uid %s, container_id %s, container_name %s\n",
		pod.GetName(),
		//pod.GetNamespace(),
		pod.GetUid(),
		container.GetId(),
		container.GetName())

	return nil
}

// StartContainer handles container start events
func (p *plugin) StartContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	log.Printf("\tContainer start event pod_name %s, pod_uid %s, container_id %s, container_name %s\n",
		pod.GetName(),
		//pod.GetNamespace(),
		pod.GetUid(),
		container.GetId(),
		container.GetName())

	return nil
}

// onClose is called when the plugin connection is closed
func (p *plugin) onClose() {
	log.Printf("Plugin connection closed\n")
	os.Exit(0)
}

func PluginStart() {
	log.Printf("plugin.PluginStart\n")
	// Create plugin instance
	p := &plugin{}

	// Create and configure stub
	opts := []stub.Option{
		stub.WithPluginName(pluginName),
		stub.WithPluginIdx(pluginIdx),
		stub.WithOnClose(p.onClose),
	}

	var err error
	p.stub, err = stub.New(p, opts...)
	if err != nil {
		log.Fatalf("Failed to create plugin stub: %v", err)
	}

	log.Printf("Starting NRI plugin name %s, index %s\n", pluginName, pluginIdx)

	// Run the plugin (blocking call)
	if err := p.stub.Run(context.Background()); err != nil {
		log.Fatalf("Plugin execution failed: %v", err)
	}

	log.Printf("Plugin stopped\n")

}
