package topology

import (
	"fmt"
	"io/fs"
	"log"
	"os"

	"sort"
	"strconv"
	"strings"
)

type Core struct {
	Cpus [2]int
}

type Chiplet struct {
	ChipletId int
	Cores     []Core
	Cpus      string
}

type NumaNode struct {
	NodeId   int
	Chiplets []Chiplet
}

type GpuStruct struct {
	Name string
	Id   int
	Numa int
}

type Topology struct {
	Cpus_total int
	SmtOn      bool
	NumaNodes  []NumaNode
	Gpus       []GpuStruct
}

func GetTopology() (Topology, error) {
	retTopology := Topology{}

	// smt?
	cpuList := fmt.Sprintf("/sys/devices/system/node/node%v/cpu%v/topology/core_cpus_list", 0, 0)
	content, _ := os.ReadFile(cpuList)
	s_content := string(content)
	s_content = strings.Replace(s_content, "\n", "", -1)
	s := strings.Split(s_content, ",")
	if len(s) > 1 {
		retTopology.SmtOn = true
	}

	nodeDir := fmt.Sprintf("/sys/devices/system/node")
	nodes, err_n := os.ReadDir(nodeDir)
	if err_n != nil {
		return retTopology, err_n
	}

	for _, nodeEntry := range nodes {
		//log.Printf(" GetTopology system/node/%s\n", nodeEntry.Name())
		_, nodeNum, nodeFound := strings.Cut(nodeEntry.Name(), "node")
		if nodeFound {
			log.Printf(" GetTopology node %s found\n", nodeNum)
			nodeId, _ := strconv.Atoi(nodeNum)
			//build node entry
			newNode := NumaNode{}
			newNode.NodeId = nodeId
			retTopology.NumaNodes = append(retTopology.NumaNodes, newNode)
			// may not be in order, onle one entry per node
			nodeIndex := nodeIndex(retTopology, nodeId)

			// now find cpus
			dir := fmt.Sprintf("/sys/devices/system/node/node%d", nodeId)
			cpus, err_c := os.ReadDir(dir)
			if err_c != nil {
				return retTopology, err_c
			}
			for _, cpuEntry := range cpus {
				fileinfo, _ := cpuEntry.Info()
				if fileinfo.Mode()&fs.ModeSymlink > 0 {
					//fmt.Printf("\t\t%q\n", cpuEntry.Name())
					_, cpuNum, cpuFound := strings.Cut(cpuEntry.Name(), "cpu")
					if cpuFound {
						//can be in any order
						log.Printf("GetTopology cpu %s found\n", cpuNum)

						cacheFile := fmt.Sprintf("/sys/devices/system/node/node0/%v/cache/index3/id", cpuEntry.Name())
						cacheIdRaw, _ := os.ReadFile(cacheFile)
						cacheIdString := strings.Replace(string(cacheIdRaw), "\n", "", -1)
						llcId, _ := strconv.Atoi(cacheIdString)
						log.Printf("\t %s, chiplet %d\n", cpuEntry.Name(), llcId)

						chipletIndex, chipletFound := chipletExists(retTopology, nodeIndex, llcId)
						if !chipletFound {
							newChiplet := Chiplet{}
							newChiplet.ChipletId = llcId

							retTopology.NumaNodes[nodeId].Chiplets = append(retTopology.NumaNodes[nodeIndex].Chiplets, newChiplet)
							chipletIndex, _ = chipletExists(retTopology, nodeIndex, llcId)

							//find cpus that belong to chiplet
							cpusFile := fmt.Sprintf("/sys/devices/system/node/node0/%v/topology/die_cpus_list", cpuEntry.Name())
							cpusRaw, _ := os.ReadFile(cpusFile)
							log.Printf("die_cpu_list %s (%d)\n", cpusRaw, chipletIndex)

							cpusString := strings.Replace(string(cpusRaw), "\n", "", -1)
							retTopology.NumaNodes[nodeIndex].Chiplets[chipletIndex].Cpus = cpusString

							//log.Printf("die_cpu_list %s\n", cpusString)
							cpus := strings.Split(cpusString, ",")
							//log.Printf("cpus %v\n", cpus)
							// double check smt
							if len(cpus) > 1 && !retTopology.SmtOn {
								log.Printf("Topology smt missmatch\n")
							}
							// get range
							work := strings.Split(cpus[0], "-")
							//log.Printf("len work %d\n", len(work))
							corea, _ := strconv.Atoi(work[0])
							coren, _ := strconv.Atoi(work[1])
							work1 := []string{}
							corez := -1
							retTopology.Cpus_total += (coren - corea) + 1
							if retTopology.SmtOn {
								work1 = strings.Split(cpus[1], "-")
								corez, _ = strconv.Atoi(work1[0])
								retTopology.Cpus_total += (coren - corea) + 1

							}

							// add cores to chiplet
							for i := 0; i < (coren-corea)+1; i++ {
								newCore := Core{}
								newCore.Cpus[0] = corea + i
								if retTopology.SmtOn {
									newCore.Cpus[1] = corez + i
								}

								retTopology.NumaNodes[nodeIndex].Chiplets[llcId].Cores = append(retTopology.NumaNodes[nodeIndex].Chiplets[llcId].Cores, newCore)
							}
						}

					}
				}
			}

		}

	}

	//sort numa nodes
	sort.Slice(retTopology.NumaNodes, func(i, j int) bool {
		return retTopology.NumaNodes[i].NodeId < retTopology.NumaNodes[j].NodeId
	})
	//for each node, sort the chiplets by ChipletId
	for n := range retTopology.NumaNodes {
		sort.Slice(retTopology.NumaNodes[n].Chiplets, func(i, j int) bool {
			return retTopology.NumaNodes[n].Chiplets[i].ChipletId < retTopology.NumaNodes[n].Chiplets[j].ChipletId
		})
	}

	//debug
	for n := 0; n < len(retTopology.NumaNodes); n++ {
		log.Printf("node %d\n", n)
		for c := 0; c < len(retTopology.NumaNodes[n].Chiplets); c++ {
			log.Printf("\tchiplet %d\n", c)
			for core := 0; core < len(retTopology.NumaNodes[n].Chiplets[c].Cores); core++ {
				if retTopology.SmtOn {
					log.Printf("\t %d;  %2d, %2d\n", core, retTopology.NumaNodes[n].Chiplets[c].Cores[core].Cpus[0], retTopology.NumaNodes[n].Chiplets[c].Cores[core].Cpus[1])
				} else {
					log.Printf("\t %d;  %2d\n", core, retTopology.NumaNodes[n].Chiplets[c].Cores[core].Cpus[0])
				}

			}
		}
	}

	retTopology.Gpus = findGpus()

	return retTopology, nil
}

func nodeIndex(t Topology, n int) int {
	index := -1
	for node := 0; node < len(t.NumaNodes); node++ {
		if n == t.NumaNodes[node].NodeId {
			index = node
			break
		}
	}
	return index
}

func chipletExists(t Topology, n int, c int) (int, bool) {
	ret := false
	index := -1

	if n < len(t.NumaNodes) {
		for chiplet := 0; chiplet < len(t.NumaNodes[n].Chiplets); chiplet++ {
			if c == t.NumaNodes[n].Chiplets[chiplet].ChipletId {
				ret = true
				index = chiplet
				break
			}
		}
	}
	return index, ret
}

func findGpus() []GpuStruct {
	gpus := []GpuStruct{}
	dir := fmt.Sprintf("/dev/dri")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return gpus
	}
	for _, gpuEntry := range entries {
		log.Printf("findGpus entry %s\n", gpuEntry.Name())
		_, gpuNum, gpuFound := strings.Cut(gpuEntry.Name(), "card")
		if gpuFound {
			log.Printf("findGpus gpuFound  num %s\n", gpuNum)
			sysDir := fmt.Sprintf("/sys/class/drm/%s/device", gpuEntry.Name())
			workFile := fmt.Sprintf("%s/vendor", sysDir)
			work, _ := os.ReadFile(workFile) //has appened newline
			log.Printf("findGpus gpuFound vendor %s\n", work)
			workFile = fmt.Sprintf("%s/numa_node", sysDir)
			work, _ = os.ReadFile(workFile) //has appened newline
			log.Printf("findGpus gpuFound numa_node %s\n", work)

			newGpu := GpuStruct{}
			newGpu.Name = gpuEntry.Name()
			newGpu.Id, _ = strconv.Atoi(gpuNum)
			newGpu.Numa, _ = strconv.Atoi(string(work))
			if newGpu.Numa < 0 {
				newGpu.Numa = 0
			}
			gpus = append(gpus, newGpu)

		}
	}
	log.Printf("findGpus gpu counr %d\n", len(gpus))
	return gpus
}
