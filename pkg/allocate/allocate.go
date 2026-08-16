package allocate

import (
	"log"
	"slices"

	"sort"
	"strconv"
	"strings"

	"github.com/adivyoseph/nri_ubb8/pkg/topology"
	"github.com/containerd/nri/pkg/api"
)

type MyContainersStruct struct {
	ContainerID   string
	ConatinerName string
	PodId         string
	PodName       string
	CpuQuanta     int64
}

type CcxStruct struct {
	CcxId      int
	GpuCard    string
	GpuCardId  int
	CpuSet     string
	Capacity   int64
	Used       int64
	Containers []MyContainersStruct
}

type NodeStruct struct {
	NodeId int
	CcXs   []CcxStruct
}

type PoolStruct struct {
	Nodes []NodeStruct
}

var _gpuPool PoolStruct
var _cpuPool PoolStruct

func Init(topo topology.Topology) {
	log.Printf("Allocate:Init start\n")

	for _, node := range topo.NumaNodes {
		newNode := NodeStruct{}
		newNode.NodeId = node.NodeId //override later
		_gpuPool.Nodes = append(_gpuPool.Nodes, newNode)
		_cpuPool.Nodes = append(_cpuPool.Nodes, newNode)
	}
	for x := 0; x < len(topo.NumaNodes); x++ {
		_gpuPool.Nodes[x].NodeId = x
		_cpuPool.Nodes[x].NodeId = x
	}
	for _, gpu := range topo.Gpus {
		//assume no duplicates
		newCcx := CcxStruct{}
		// allocate ccx later
		newCcx.GpuCard = gpu.Name
		newCcx.GpuCardId = gpu.Id
		if gpu.Numa >= len(_gpuPool.Nodes) {
			log.Fatal(-2)
		}

		_gpuPool.Nodes[gpu.Numa].CcXs = append(_gpuPool.Nodes[gpu.Numa].CcXs, newCcx)
	}
	for x := 0; x < len(topo.NumaNodes); x++ {
		// for gpu pool by numa node, sort ccxs by gpuid
		sort.Slice(_gpuPool.Nodes[x].CcXs, func(i, j int) bool {
			return _gpuPool.Nodes[x].CcXs[i].GpuCardId < _gpuPool.Nodes[x].CcXs[j].GpuCardId
		})
	}

	//create full ccx topology, then remove reserved and gpu ccx's
	for _, node := range topo.NumaNodes {
		nodeIndex := node.NodeId
		//ad bland ccxs
		for c := 0; c < len(node.Chiplets); c++ {
			newCcx := CcxStruct{}
			newCcx.CcxId = node.Chiplets[c].ChipletId
			newCcx.CpuSet = node.Chiplets[c].Cpus
			newCcx.Capacity = int64(len(node.Chiplets[c].Cores) * 1024)
			if topo.SmtOn {
				newCcx.Capacity = newCcx.Capacity * 2
			}

			_cpuPool.Nodes[nodeIndex].CcXs = append(_cpuPool.Nodes[nodeIndex].CcXs, newCcx)
		}
	}

	// remove reserved cores
	// TDO get details from kubeletconfig
	// for now hard code to chiplet zero

	// for each numa node remove ccs from cpu pool and give to gpu pool
	for n := 0; n < len(_gpuPool.Nodes); n++ {
		gpusOnNode := len(_gpuPool.Nodes[n].CcXs)
		gpuCcx := 0
		cpuLastCcx := len(_cpuPool.Nodes[n].CcXs) - gpusOnNode
		for cpuCcx := cpuLastCcx; cpuCcx < len(_cpuPool.Nodes[n].CcXs); cpuCcx++ {
			//transfer ccx details to gpu pol entry
			_gpuPool.Nodes[n].CcXs[gpuCcx].CcxId = _cpuPool.Nodes[n].CcXs[cpuCcx].CcxId
			_gpuPool.Nodes[n].CcXs[gpuCcx].CpuSet = _cpuPool.Nodes[n].CcXs[cpuCcx].CpuSet
			_gpuPool.Nodes[n].CcXs[gpuCcx].Capacity = _cpuPool.Nodes[n].CcXs[cpuCcx].Capacity

		}
		_cpuPool.Nodes[n].CcXs = slices.Delete(_cpuPool.Nodes[n].CcXs, cpuLastCcx, cpuLastCcx+gpusOnNode)
	}

	//debug
	log.Printf("GPU pool\n")
	for x := 0; x < len(topo.NumaNodes); x++ {
		log.Printf("node %d (nodeid %d)\n", x, _gpuPool.Nodes[x].NodeId)
		for c := 0; c < len(_cpuPool.Nodes[x].CcXs); c++ {
			log.Printf("  ccx %d card %s cpus %s\n", _gpuPool.Nodes[x].CcXs[c].CcxId, _gpuPool.Nodes[x].CcXs[c].GpuCard, _gpuPool.Nodes[x].CcXs[c].CpuSet)

		}

	}
	log.Printf("CPU pool\n")
	for x := 0; x < len(topo.NumaNodes); x++ {
		log.Printf("node %d (nodeid %d)\n", x, _cpuPool.Nodes[x].NodeId)
		for c := 0; c < len(_cpuPool.Nodes[x].CcXs); c++ {
			log.Printf("  ccx %d cpus %s\n", _cpuPool.Nodes[x].CcXs[c].CcxId, _cpuPool.Nodes[x].CcXs[c].CpuSet)

		}

	}

}

func ReserveCard(pod *api.PodSandbox, container *api.Container, card string, smt int) (string, string) {
	var cpuset string
	numa := "0"
	gpuFound := false

	for n := 0; n < len(_gpuPool.Nodes) && !gpuFound; n++ {
		for c := 0; c < len(_gpuPool.Nodes[n].CcXs); c++ {
			if card == _gpuPool.Nodes[n].CcXs[c].GpuCard {
				cpuset = _gpuPool.Nodes[n].CcXs[c].CpuSet
				//smt off check
				if smt == 1 {
					cpus := strings.Split(cpuset, ",")
					cpuset = cpus[0]
				}

				numa = strconv.Itoa(n)
				//set state
				newContainer := MyContainersStruct{}
				newContainer.ConatinerName = container.Name
				newContainer.ContainerID = container.Id
				newContainer.PodId = pod.Id
				newContainer.PodName = pod.GetName()
				newContainer.CpuQuanta = int64(container.Linux.Resources.Cpu.Shares.GetValue())

				_gpuPool.Nodes[n].CcXs[c].Containers = append(_gpuPool.Nodes[n].CcXs[c].Containers, newContainer)

				gpuFound = true
				break
			}
		}
	}

	return cpuset, numa

}

func AddContainerSharedPool(pod *api.PodSandbox, container *api.Container) (string, string) {
	//assume packed for now
	//TODO add support for other binpacking strategies
	var cpuset string
	numa := ""
	notFound := true
	quantaAsk := int64(container.Linux.Resources.Cpu.Shares.GetValue())
	if quantaAsk > _cpuPool.Nodes[0].CcXs[0].Capacity {
		quantaAsk = _cpuPool.Nodes[0].CcXs[0].Capacity
	}
	//try to fit in one ccx
	for n := 0; n < len(_cpuPool.Nodes) && notFound; n++ {
		for c := 0; c < len(_cpuPool.Nodes[n].CcXs) && notFound; c++ {
			if quantaAsk <= _cpuPool.Nodes[n].CcXs[c].Capacity-_cpuPool.Nodes[n].CcXs[c].Used {
				//found a slot
				numa = strconv.Itoa(n)
				//set state
				newContainer := MyContainersStruct{}
				newContainer.ConatinerName = container.Name
				newContainer.ContainerID = container.Id
				newContainer.PodId = pod.Id
				newContainer.PodName = pod.GetName()
				newContainer.CpuQuanta = quantaAsk
				_cpuPool.Nodes[n].CcXs[c].Used = +quantaAsk

				_gpuPool.Nodes[n].CcXs[c].Containers = append(_gpuPool.Nodes[n].CcXs[c].Containers, newContainer)

				notFound = false
				break

			}
		}
	}

	return cpuset, numa
}

func RemoveContainer(pod *api.PodSandbox, container *api.Container) {
	notFound := true
	for n := 0; n < len(_cpuPool.Nodes) && notFound; n++ {
		for c := 0; c < len(_cpuPool.Nodes[n].CcXs) && notFound; c++ {
			for containerIndex := 0; containerIndex < len(_cpuPool.Nodes[n].CcXs[c].Containers); containerIndex++ {
				if container.Id == _cpuPool.Nodes[n].CcXs[c].Containers[containerIndex].ContainerID {
					_cpuPool.Nodes[n].CcXs[c].Used = -_cpuPool.Nodes[n].CcXs[c].Containers[containerIndex].CpuQuanta
					if _cpuPool.Nodes[n].CcXs[c].Used < 0 {
						_cpuPool.Nodes[n].CcXs[c].Used = 0
					}
					_cpuPool.Nodes[n].CcXs[c].Containers = slices.Delete(_cpuPool.Nodes[n].CcXs[c].Containers, containerIndex, containerIndex+1)
					notFound = false
					break
				}
			}
		}
	}
	if notFound {
		for n := 0; n < len(_gpuPool.Nodes) && notFound; n++ {
			for c := 0; c < len(_gpuPool.Nodes[n].CcXs) && notFound; c++ {
				for containerIndex := 0; containerIndex < len(_gpuPool.Nodes[n].CcXs[c].Containers); containerIndex++ {
					if container.Id == _gpuPool.Nodes[n].CcXs[c].Containers[containerIndex].ContainerID {
						_gpuPool.Nodes[n].CcXs[c].Containers = slices.Delete(_gpuPool.Nodes[n].CcXs[c].Containers, containerIndex, containerIndex+1)
						notFound = true
						break
					}
				}
			}
		}
	}

}
